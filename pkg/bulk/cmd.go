package bulk

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
)

type Runner interface {
	Run(list *kapi.List, namespace string) []error
}

// AfterFunc takes an info and an error, and returns true if processing should stop.
type AfterFunc func(*unstructured.Unstructured, error) bool

// OpFunc takes the provided info and attempts to perform the operation
type OpFunc func(obj *unstructured.Unstructured, namespace string) (*unstructured.Unstructured, error)

// RetryFunc can retry the operation a single time by returning a non-nil object.
// TODO: make this a more general retry "loop" function rather than one time.
type RetryFunc func(obj *unstructured.Unstructured, err error) *unstructured.Unstructured

// IgnoreErrorFunc provides a way to filter errors during the Bulk.Run.  If this function returns
// true the error will NOT be added to the slice of errors returned by Bulk.Run.
//
// This may be used in conjunction with
// BulkAction.WithMessageAndPrefix if you are reporting some errors as warnings.
type IgnoreErrorFunc func(e error) bool

// Bulk provides helpers for iterating over a list of items
type Bulk struct {
	Scheme *runtime.Scheme

	Op          OpFunc
	After       AfterFunc
	Retry       RetryFunc
	IgnoreError IgnoreErrorFunc
}

// Run attempts to create each item generically, gathering all errors in the
// event a failure occurs. The contents of list will be updated to include the
// version from the server.
// For now, run will do a conversion from internal or versioned, to version, then to unstructured.
func (b *Bulk) Run(list *kapi.List, namespace string) []error {
	after := b.After
	if after == nil {
		after = func(*unstructured.Unstructured, error) bool { return false }
	}
	ignoreError := b.IgnoreError
	if ignoreError == nil {
		ignoreError = func(e error) bool { return false }
	}

	errs := []error{}
	for i := range list.Items {
		item := list.Items[i].DeepCopyObject()
		unstructuredObj, ok := item.(*unstructured.Unstructured)
		if !ok {
			var err error
			converter := runtime.ObjectConvertor(b.Scheme)
			groupVersioner := runtime.GroupVersioner(schema.GroupVersions(b.Scheme.PrioritizedVersionsAllGroups()))
			versionedObj, err := converter.ConvertToVersion(item, groupVersioner)
			if err != nil {
				errs = append(errs, err)
				if after(nil, err) {
					break
				}
				continue
			}
			unstructuredObj = &unstructured.Unstructured{}
			unstructuredObj.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(versionedObj)
			if err != nil {
				errs = append(errs, err)
				if after(nil, err) {
					break
				}
				continue
			}
		}

		unstructuredObj, err := b.Op(unstructuredObj, namespace)
		if err != nil && b.Retry != nil {
			if unstructuredObj = b.Retry(unstructuredObj, err); unstructuredObj != nil {
				unstructuredObj, err = b.Op(unstructuredObj, namespace)
			}
		}
		if err != nil {
			if !ignoreError(err) {
				errs = append(errs, err)
			}
			if after(unstructuredObj, err) {
				break
			}
			continue
		}
		list.Items[i] = unstructuredObj
		if after(unstructuredObj, nil) {
			break
		}
	}
	return errs
}

func NewPrintNameOrErrorAfterIndent(short bool, operation string, out, errs io.Writer, dryRun bool, indent string, prefixForError PrefixForError) AfterFunc {
	return func(obj *unstructured.Unstructured, err error) bool {
		if err == nil {
			fmt.Fprintf(out, indent)
			printSuccess(short, out, obj.GroupVersionKind(), obj.GetName(), dryRun, operation)
		} else {
			fmt.Fprintf(errs, "%s%s: %v\n", indent, prefixForError(err), err)
		}
		return false
	}
}

func printSuccess(shortOutput bool, out io.Writer, gvk schema.GroupVersionKind, name string, dryRun bool, operation string) {
	kindString := gvk.Kind
	if len(gvk.Group) > 0 {
		kindString = gvk.Kind + "." + gvk.Group
	}
	kindString = strings.ToLower(kindString)

	dryRunMsg := ""
	if dryRun {
		dryRunMsg = " (dry run)"
	}
	if shortOutput {
		// -o name: prints resource/name
		if len(kindString) > 0 {
			fmt.Fprintf(out, "%s/%s\n", kindString, name)
		} else {
			fmt.Fprintf(out, "%s\n", name)
		}
	} else {
		// understandable output by default
		if len(kindString) > 0 {
			fmt.Fprintf(out, "%s \"%s\" %s%s\n", kindString, name, operation, dryRunMsg)
		} else {
			fmt.Fprintf(out, "\"%s\" %s%s\n", name, operation, dryRunMsg)
		}
	}
}

func NewPrintErrorAfter(errs io.Writer, prefixForError PrefixForError) func(*unstructured.Unstructured, error) bool {
	return func(obj *unstructured.Unstructured, err error) bool {
		if err != nil {
			fmt.Fprintf(errs, "%s: %v\n", prefixForError(err), err)
		}
		return false
	}
}

func HaltOnError(fn AfterFunc) AfterFunc {
	return func(obj *unstructured.Unstructured, err error) bool {
		if fn(obj, err) || err != nil {
			return true
		}
		return false
	}
}

type Creator struct {
	Client     dynamic.Interface
	RESTMapper meta.RESTMapper
}

// Create is the default create operation for a generic resource.
func (c Creator) Create(obj *unstructured.Unstructured, namespace string) (*unstructured.Unstructured, error) {
	if len(obj.GetNamespace()) > 0 {
		namespace = obj.GetNamespace()
	}
	mapping, err := c.RESTMapper.RESTMapping(obj.GroupVersionKind().GroupKind(), obj.GroupVersionKind().Version)
	if err != nil {
		return nil, err
	}
	if mapping.Scope.Name() == meta.RESTScopeNameRoot {
		namespace = ""
	}

	return c.Client.Resource(mapping.Resource).Namespace(namespace).Create(obj)
}

func NoOp(obj *unstructured.Unstructured, namespace string) (*unstructured.Unstructured, error) {
	return obj, nil
}

func labelSuffix(set map[string]string) string {
	if len(set) == 0 {
		return ""
	}
	return fmt.Sprintf(" with label %s", labels.SelectorFromSet(set).String())
}

func CreateMessage(labels map[string]string) string {
	return fmt.Sprintf("Creating resources%s", labelSuffix(labels))
}

type BulkAction struct {
	// required setup
	Bulk Bulk

	// flags
	Output      string
	DryRun      bool
	StopOnError bool

	// output modifiers
	Action string

	genericclioptions.IOStreams
}

// BindForAction sets flags on this action for when setting -o should only change how the operation results are displayed.
// Passing -o is changing the default output format.
func (b *BulkAction) BindForAction(flags *pflag.FlagSet) {
	flags.StringVarP(&b.Output, "output", "o", "", "Output mode. Use \"-o name\" for shorter output (resource/name).")
	flags.BoolVar(&b.DryRun, "dry-run", false, "If true, show the result of the operation without performing it.")
}

// BindForOutput sets flags on this action for when setting -o will not execute the action (the point of the action is
// primarily to generate the output). Passing -o is asking for output, not execution.
func (b *BulkAction) BindForOutput(flags *pflag.FlagSet, skippedFlags ...string) {
	skipped := sets.NewString(skippedFlags...)

	if !skipped.Has("output") {
		flags.StringVarP(&b.Output, "output", "o", "", "Output results as yaml or json instead of executing, or use name for succint output (resource/name).")
	}
	if !skipped.Has("dry-run") {
		flags.BoolVar(&b.DryRun, "dry-run", false, "If true, show the result of the operation without performing it.")
	}
	if !skipped.Has("no-headers") {
		flags.Bool("no-headers", false, "Omit table headers for default output.")
		flags.MarkHidden("no-headers")
	}

	// we really want to call the AddNonDeprecatedPrinterFlags method, but that is broken since it doesn't bind vars
	if !skipped.Has("show-labels") {
		flags.Bool("show-labels", false, "When printing, show all labels as the last column (default hide labels column)")
	}
	if !skipped.Has("template") {
		flags.String("template", "", "Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].")
		cobra.MarkFlagFilename(flags, "template")
	}
	if !skipped.Has("sort-by") {
		flags.String("sort-by", "", "If non-empty, sort list types using this field specification.  The field specification is expressed as a JSONPath expression (e.g. '{.metadata.name}'). The field in the API resource specified by this JSONPath expression must be an integer or a string.")
	}
	if !skipped.Has("show-all") {
		flags.BoolP("show-all", "a", false, "When printing, show all resources (default hide terminated pods.)")
	}
}

// Compact sets the output to a minimal set
func (b *BulkAction) Compact() {
	b.Output = "compact"
}

// ShouldPrint returns true if an external printer should handle this action instead of execution.
func (b *BulkAction) ShouldPrint() bool {
	return b.Output != "" && b.Output != "name" && b.Output != "compact"
}

func (b *BulkAction) Verbose() bool {
	return b.Output == ""
}

func (b *BulkAction) DefaultIndent() string {
	if b.Verbose() {
		return "    "
	}
	return ""
}

// PrefixForError allows customization of the prefix that will be printed for any error that occurs in the BulkAction.
type PrefixForError func(e error) string

func (b BulkAction) WithMessageAndPrefix(action, individual string, prefixForError PrefixForError) Runner {
	b.Action = action
	switch {
	// TODO: this should be b printer
	case b.Output == "":
		b.Bulk.After = NewPrintNameOrErrorAfterIndent(false, individual, b.Out, b.ErrOut, b.DryRun, b.DefaultIndent(), prefixForError)
	// TODO: needs to be unified with the name printer (incremental vs exact execution), possibly by creating b synthetic printer?
	case b.Output == "name":
		b.Bulk.After = NewPrintNameOrErrorAfterIndent(true, individual, b.Out, b.ErrOut, b.DryRun, b.DefaultIndent(), prefixForError)
	default:
		b.Bulk.After = NewPrintErrorAfter(b.ErrOut, prefixForError)
		if b.StopOnError {
			b.Bulk.After = HaltOnError(b.Bulk.After)
		}
	}
	return &b
}

func (b BulkAction) WithMessage(action, individual string) Runner {
	return b.WithMessageAndPrefix(action, individual, func(e error) string { return "error" })
}

func (b *BulkAction) Run(list *kapi.List, namespace string) []error {
	run := b.Bulk

	if b.Verbose() {
		fmt.Fprintf(b.Out, "--> %s ...\n", b.Action)
	}

	var modifier string
	if b.DryRun {
		run.Op = NoOp
		modifier = " (dry run)"
	}

	errs := run.Run(list, namespace)
	if b.Verbose() {
		if len(errs) == 0 {
			fmt.Fprintf(b.Out, "--> Success%s\n", modifier)
		} else {
			fmt.Fprintf(b.Out, "--> Failed%s\n", modifier)
		}
	}
	return errs
}
