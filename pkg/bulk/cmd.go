package bulk

import (
	"fmt"
	"io"
	"os"
	"path"
	goruntime "runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/version"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Runner interface {
	Run(list *kapi.List, namespace string) []error
}

// AfterFunc takes an info and an error, and returns true if processing should stop.
type AfterFunc func(*resource.Info, error) bool

// OpFunc takes the provided info and attempts to perform the operation
type OpFunc func(info *resource.Info, namespace string, obj runtime.Object) (runtime.Object, error)

// RetryFunc can retry the operation a single time by returning a non-nil object.
// TODO: make this a more general retry "loop" function rather than one time.
type RetryFunc func(info *resource.Info, err error) runtime.Object

// Mapper is an interface testability that is equivalent to resource.Mapper
type Mapper interface {
	meta.RESTMapper
	InfoForObject(obj runtime.Object, preferredGVKs []schema.GroupVersionKind) (*resource.Info, error)
}

// IgnoreErrorFunc provides a way to filter errors during the Bulk.Run.  If this function returns
// true the error will NOT be added to the slice of errors returned by Bulk.Run.
//
// This may be used in conjunction with
// BulkAction.WithMessageAndPrefix if you are reporting some errors as warnings.
type IgnoreErrorFunc func(e error) bool

// Bulk provides helpers for iterating over a list of items
type Bulk struct {
	Mapper        Mapper
	DynamicMapper Mapper

	// PreferredSerializationOrder take a list of GVKs to decide how to serialize out the individual list items
	// It allows partial values, so you specify just groups or versions as a for instance
	PreferredSerializationOrder []schema.GroupVersionKind

	Op          OpFunc
	After       AfterFunc
	Retry       RetryFunc
	IgnoreError IgnoreErrorFunc
}

// Run attempts to create each item generically, gathering all errors in the
// event a failure occurs. The contents of list will be updated to include the
// version from the server.
func (b *Bulk) Run(list *kapi.List, namespace string) []error {
	after := b.After
	if after == nil {
		after = func(*resource.Info, error) bool { return false }
	}
	ignoreError := b.IgnoreError
	if ignoreError == nil {
		ignoreError = func(e error) bool { return false }
	}

	errs := []error{}
	for i, item := range list.Items {
		var info *resource.Info
		var err error
		if _, ok := item.(*unstructured.Unstructured); ok {
			info, err = b.DynamicMapper.InfoForObject(item, b.getPreferredSerializationOrder())
		} else {
			info, err = b.Mapper.InfoForObject(item, b.getPreferredSerializationOrder())
		}
		if err != nil {
			errs = append(errs, err)
			if after(info, err) {
				break
			}
			continue
		}
		obj, err := b.Op(info, namespace, item)
		if err != nil && b.Retry != nil {
			if obj = b.Retry(info, err); obj != nil {
				obj, err = b.Op(info, namespace, obj)
			}
		}
		if err != nil {
			if !ignoreError(err) {
				errs = append(errs, err)
			}
			if after(info, err) {
				break
			}
			continue
		}
		info.Refresh(obj, true)
		list.Items[i] = obj
		if after(info, nil) {
			break
		}
	}
	return errs
}

func (b *Bulk) getPreferredSerializationOrder() []schema.GroupVersionKind {
	if len(b.PreferredSerializationOrder) > 0 {
		return b.PreferredSerializationOrder
	}
	// it seems that the underlying impl expects to have at least one, even though this
	// logically means something different.
	return []schema.GroupVersionKind{{Group: ""}}
}

// ClientMapperFromConfig returns a ClientMapper suitable for Bulk operations.
// TODO: copied from
// pkg/cmd/util/clientcmd/factory_object_mapping.go#ClientForMapping and
// vendor/k8s.io/kubernetes/pkg/kubectl/cmd/util/factory_object_mapping.go#ClientForMapping
func ClientMapperFromConfig(config *rest.Config) resource.ClientMapperFunc {
	return resource.ClientMapperFunc(func(mapping *meta.RESTMapping) (resource.RESTClient, error) {
		configCopy := *config

		if latest.OriginKind(mapping.GroupVersionKind) {
			if err := SetLegacyOpenShiftDefaults(&configCopy); err != nil {
				return nil, err
			}
			configCopy.APIPath = "/apis"
			if mapping.GroupVersionKind.Group == kapi.GroupName {
				configCopy.APIPath = "/oapi"
			}
			gv := mapping.GroupVersionKind.GroupVersion()
			configCopy.GroupVersion = &gv
			return rest.RESTClientFor(&configCopy)
		}

		if err := unversioned.SetKubernetesDefaults(&configCopy); err != nil {
			return nil, err
		}
		gvk := mapping.GroupVersionKind
		switch gvk.Group {
		case kapi.GroupName:
			configCopy.APIPath = "/api"
		default:
			configCopy.APIPath = "/apis"
		}
		gv := gvk.GroupVersion()
		configCopy.GroupVersion = &gv
		return rest.RESTClientFor(&configCopy)
	})
}

// PreferredSerializationOrder returns the preferred ordering via discovery. If anything fails, it just
// returns a list of with the empty (legacy) group
func PreferredSerializationOrder(client discovery.DiscoveryInterface) []schema.GroupVersionKind {
	ret := []schema.GroupVersionKind{{Group: ""}}
	if client == nil {
		return ret
	}

	groups, err := client.ServerGroups()
	if err != nil {
		return ret
	}

	resources, err := client.ServerResources()
	if err != nil {
		return ret
	}

	// add resources with kinds first, then groupversions second as a fallback.  Not all server versions provide kinds
	// we have to specify individual kinds since our local scheme may have kinds not present on the server
	for _, resourceList := range resources {
		for _, resource := range resourceList.APIResources {
			// if this is a sub resource, skip it
			if strings.Contains(resource.Name, "/") {
				continue
			}
			// if there is no kind, skip it
			if len(resource.Kind) == 0 {
				continue
			}
			gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
			if err != nil {
				continue
			}
			ret = append(ret, gv.WithKind(resource.Kind))
		}
	}

	// we actually have to get to the granularity of versions because the server may not support the same versions
	// in a group that we have locally.
	for _, group := range groups.Groups {
		for _, version := range group.Versions {
			ret = append(ret, schema.GroupVersionKind{Group: group.Name, Version: version.Version})
		}
	}
	return ret
}

func NewPrintNameOrErrorAfterIndent(mapper meta.RESTMapper, short bool, operation string, out, errs io.Writer, dryRun bool, indent string, prefixForError PrefixForError) AfterFunc {
	return func(info *resource.Info, err error) bool {
		if err == nil {
			fmt.Fprintf(out, indent)
			printSuccess(mapper, short, out, info.Mapping.Resource, info.Name, dryRun, operation)
		} else {
			fmt.Fprintf(errs, "%s%s: %v\n", indent, prefixForError(err), err)
		}
		return false
	}
}

func printSuccess(mapper meta.RESTMapper, shortOutput bool, out io.Writer, resource, name string, dryRun bool, operation string) {
	resource, _ = mapper.ResourceSingularizer(resource)
	dryRunMsg := ""
	if dryRun {
		dryRunMsg = " (dry run)"
	}
	if shortOutput {
		// -o name: prints resource/name
		if len(resource) > 0 {
			fmt.Fprintf(out, "%s/%s\n", resource, name)
		} else {
			fmt.Fprintf(out, "%s\n", name)
		}
	} else {
		// understandable output by default
		if len(resource) > 0 {
			fmt.Fprintf(out, "%s \"%s\" %s%s\n", resource, name, operation, dryRunMsg)
		} else {
			fmt.Fprintf(out, "\"%s\" %s%s\n", name, operation, dryRunMsg)
		}
	}
}

func NewPrintErrorAfter(mapper meta.RESTMapper, errs io.Writer, prefixForError PrefixForError) func(*resource.Info, error) bool {
	return func(info *resource.Info, err error) bool {
		if err != nil {
			fmt.Fprintf(errs, "%s: %v\n", prefixForError(err), err)
		}
		return false
	}
}

func HaltOnError(fn AfterFunc) AfterFunc {
	return func(info *resource.Info, err error) bool {
		if fn(info, err) || err != nil {
			return true
		}
		return false
	}
}

// Create is the default create operation for a generic resource.
func Create(info *resource.Info, namespace string, obj runtime.Object) (runtime.Object, error) {
	if len(info.Namespace) > 0 {
		namespace = info.Namespace
	}
	return resource.NewHelper(info.Client, info.Mapping).Create(namespace, false, obj)
}

func NoOp(info *resource.Info, namespace string, obj runtime.Object) (runtime.Object, error) {
	return info.Object, nil
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
	Bulk        Bulk
	Out, ErrOut io.Writer

	// flags
	Output      string
	DryRun      bool
	StopOnError bool

	// output modifiers
	Action string
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
		b.Bulk.After = NewPrintNameOrErrorAfterIndent(b.Bulk.Mapper, false, individual, b.Out, b.ErrOut, b.DryRun, b.DefaultIndent(), prefixForError)
	// TODO: needs to be unified with the name printer (incremental vs exact execution), possibly by creating b synthetic printer?
	case b.Output == "name":
		b.Bulk.After = NewPrintNameOrErrorAfterIndent(b.Bulk.Mapper, true, individual, b.Out, b.ErrOut, b.DryRun, b.DefaultIndent(), prefixForError)
	default:
		b.Bulk.After = NewPrintErrorAfter(b.Bulk.Mapper, b.ErrOut, prefixForError)
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

// SetLegacyOpenShiftDefaults sets the default settings on the passed client configuration for legacy usage
func SetLegacyOpenShiftDefaults(config *rest.Config) error {
	if len(config.UserAgent) == 0 {
		config.UserAgent = defaultOpenShiftUserAgent()
	}
	if config.GroupVersion == nil {
		// Clients default to the preferred code API version
		groupVersionCopy := latest.Version
		config.GroupVersion = &groupVersionCopy
	}
	if config.APIPath == "" || config.APIPath == "/api" {
		config.APIPath = "/oapi"
	}
	if config.NegotiatedSerializer == nil {
		config.NegotiatedSerializer = legacyscheme.Codecs
	}
	return nil
}

func defaultOpenShiftUserAgent() string {
	commit := version.Get().GitCommit
	if len(commit) > 7 {
		commit = commit[:7]
	}
	if len(commit) == 0 {
		commit = "unknown"
	}
	version := version.Get().GitVersion
	seg := strings.SplitN(version, "-", 2)
	version = seg[0]
	return fmt.Sprintf("%s/%s (%s/%s) openshift/%s", path.Base(os.Args[0]), version, goruntime.GOOS, goruntime.GOARCH, commit)
}
