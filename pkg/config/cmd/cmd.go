package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/pflag"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
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
	InfoForObject(obj runtime.Object, preferredGVKs []unversioned.GroupVersionKind) (*resource.Info, error)
}

// Bulk provides helpers for iterating over a list of items
type Bulk struct {
	Mapper Mapper

	Op    OpFunc
	After AfterFunc
	Retry RetryFunc
}

// Create attempts to create each item generically, gathering all errors in the
// event a failure occurs. The contents of list will be updated to include the
// version from the server.
func (b *Bulk) Run(list *kapi.List, namespace string) []error {
	after := b.After
	if after == nil {
		after = func(*resource.Info, error) bool { return false }
	}

	errs := []error{}
	for i, item := range list.Items {
		info, err := b.Mapper.InfoForObject(item, nil)
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
			errs = append(errs, err)
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

func NewPrintNameOrErrorAfter(mapper meta.RESTMapper, short bool, operation string, out, errs io.Writer) AfterFunc {
	return NewPrintNameOrErrorAfterIndent(mapper, short, operation, out, errs, "")
}

func NewPrintNameOrErrorAfterIndent(mapper meta.RESTMapper, short bool, operation string, out, errs io.Writer, indent string) AfterFunc {
	return func(info *resource.Info, err error) bool {
		if err == nil {
			fmt.Fprintf(out, indent)
			cmdutil.PrintSuccess(mapper, short, out, info.Mapping.Resource, info.Name, operation)
		} else {
			fmt.Fprintf(errs, "%serror: %v\n", indent, err)
		}
		return false
	}
}

func NewPrintErrorAfter(mapper meta.RESTMapper, errs io.Writer) func(*resource.Info, error) bool {
	return func(info *resource.Info, err error) bool {
		if err != nil {
			fmt.Fprintf(errs, "error: %v\n", err)
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
func (b *BulkAction) BindForOutput(flags *pflag.FlagSet) {
	flags.StringVarP(&b.Output, "output", "o", "", "Output results as yaml or json instead of executing, or use name for succint output (resource/name).")
	flags.BoolVar(&b.DryRun, "dry-run", false, "If true, show the result of the operation without performing it.")
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

func (b BulkAction) WithMessage(action, individual string) Runner {
	b.Action = action
	switch {
	// TODO: this should be b printer
	case b.Output == "":
		b.Bulk.After = NewPrintNameOrErrorAfterIndent(b.Bulk.Mapper, false, individual, b.Out, b.ErrOut, b.DefaultIndent())
	// TODO: needs to be unified with the name printer (incremental vs exact execution), possibly by creating b synthetic printer?
	case b.Output == "name":
		b.Bulk.After = NewPrintNameOrErrorAfterIndent(b.Bulk.Mapper, true, individual, b.Out, b.ErrOut, b.DefaultIndent())
	default:
		b.Bulk.After = NewPrintErrorAfter(b.Bulk.Mapper, b.ErrOut)
		if b.StopOnError {
			b.Bulk.After = HaltOnError(b.Bulk.After)
		}
	}
	return &b
}

func (b *BulkAction) Run(list *kapi.List, namespace string) []error {
	run := b.Bulk

	if b.Verbose() {
		fmt.Fprintf(b.Out, "--> %s ...\n", b.Action)
	}

	var modifier string
	if b.DryRun {
		run.Op = NoOp
		modifier = " (DRY RUN)"
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
