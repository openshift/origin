package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	idling "github.com/openshift/service-idler/pkg/apis/idling/v1alpha2"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"

	idlingutil "github.com/openshift/origin/pkg/idling"
)

var (
	idleLong = templates.LongDesc(`
		Idle and unidle idlers

		Idling will cause all target scalable resources of the given idler to be scaled
		to zero, with their previous scales saved.

		Unidling will restore the target scalables to their previous scales.
		Network traffic to any of the trigger services of the idler will also cause unidling.`)

	idleExample = templates.Examples(`
		# Set the main-app idler to idle
		$ %[1]s idle main-app
		# Set the main-app idler to unidle
		$ %[1]s idle --unidle main-app
		`)
)

// NewCmdIdle implements the OpenShift cli idle command
func NewCmdIdle(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	opts := &IdleOptions{
		out:    streams.Out,
		errOut: streams.ErrOut,
	}

	cmd := &cobra.Command{
		Use:     "idle (IDLER_NAME... | -l label | --all | -f FILENAME) [flags]",
		Short:   "Idle and unidle idlers",
		Long:    idleLong,
		Example: fmt.Sprintf(idleExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, cmd, args))
			kcmdutil.CheckErr(opts.Run(f, cmd, args))
		},
	}

	kcmdutil.AddFilenameOptionFlags(cmd, &opts.FilenameOptions, "Filename, directory, or URL to a file identifying the resource to get from a server.")
	cmd.Flags().StringVarP(&opts.LabelSelector, "selector", "l", opts.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Flags().BoolVar(&opts.AllNamespaces, "all-namespaces", opts.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().BoolVar(&opts.Unidle, "unidle", opts.Unidle, "If present, unidle the given idlers.")

	return cmd
}

type IdleOptions struct {
	out, errOut io.Writer
	patch       []byte

	resource.FilenameOptions
	LabelSelector     string
	AllNamespaces     bool
	Namespace         string
	ExplicitNamespace bool

	Unidle bool
}

func (o *IdleOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error
	o.Namespace, o.ExplicitNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	if o.AllNamespaces {
		o.ExplicitNamespace = false
	}

	if o.Unidle {
		o.patch = idlingutil.UnidlePatchData
	} else {
		o.patch = idlingutil.IdlePatchData
	}

	return nil
}

func (o *IdleOptions) Run(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	groupRes := idling.Resource("idlers")
	r := f.NewBuilder().
		Unstructured().
		NamespaceParam(o.Namespace).DefaultNamespace().AllNamespaces(o.AllNamespaces).
		FilenameParam(o.ExplicitNamespace, &o.FilenameOptions).
		LabelSelectorParam(o.LabelSelector).
		ResourceNames(groupRes.String(), args...).
		ContinueOnError().
		Flatten().
		SingleResourceType().
		Do()

	verb := "idling"
	pastVerb := "idled"
	if o.Unidle {
		verb = "unidling"
		pastVerb = "unidled"
	}
	return r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		metadata, err := meta.Accessor(info.Object)
		if err != nil {
			return kcmdutil.AddSourceToErr(verb, info.Source, err)
		}

		helper := resource.NewHelper(info.Client, info.Mapping)
		_, err = helper.Patch(metadata.GetNamespace(), metadata.GetName(), types.JSONPatchType, o.patch)
		if err != nil {
			return kcmdutil.AddSourceToErr(verb, info.Source, err)
		}
		kcmdutil.PrintSuccess(false, o.out, info.Object, false, pastVerb)
		return nil
	})
}
