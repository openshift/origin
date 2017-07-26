package create

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

const ImageStreamRecommendedName = "imagestream"

var (
	imageStreamLong = templates.LongDesc(`
		Create a new image stream

		Image streams allow you to track, tag, and import images from other registries. They also define an
		access controlled destination that you can push images to. An image stream can reference images
		from many different registries and control how those images are referenced by pods, deployments,
		and builds.

		If --resolve-local is passed, the image stream will be used as the source when pods reference
		it by name. For example, if stream 'mysql' resolves local names, a pod that points to
		'mysql:latest' will use the image the image stream points to under the "latest" tag.`)

	imageStreamExample = templates.Examples(`
		# Create a new image stream
  	%[1]s mysql`)
)

type CreateImageStreamOptions struct {
	IS     *imageapi.ImageStream
	Client client.ImageStreamsNamespacer

	DryRun bool

	Mapper       meta.RESTMapper
	OutputFormat string
	Out          io.Writer
	Printer      ObjectPrinter
}

// NewCmdCreateImageStream is a macro command to create a new image stream
func NewCmdCreateImageStream(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &CreateImageStreamOptions{
		Out: out,
		IS: &imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       imageapi.ImageStreamSpec{},
		},
	}

	cmd := &cobra.Command{
		Use:     name + " NAME",
		Short:   "Create a new empty image stream.",
		Long:    imageStreamLong,
		Example: fmt.Sprintf(imageStreamExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd, f, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
		Aliases: []string{"is"},
	}

	cmd.Flags().BoolVar(&o.IS.Spec.LookupPolicy.Local, "lookup-local", false, "If true, the image stream will be the source for any top-level image reference in this project.")
	cmdutil.AddPrinterFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)
	return cmd
}

func (o *CreateImageStreamOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	o.DryRun = cmdutil.GetFlagBool(cmd, "dry-run")

	switch len(args) {
	case 0:
		return fmt.Errorf("image stream name is required")
	case 1:
		o.IS.Name = args[0]
	default:
		return fmt.Errorf("exactly one argument (name) is supported, not: %v", args)
	}

	var err error
	o.IS.Namespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.Client, _, err = f.Clients()
	if err != nil {
		return err
	}

	o.Mapper, _ = f.Object()
	o.OutputFormat = cmdutil.GetFlagString(cmd, "output")

	o.Printer = func(obj runtime.Object, out io.Writer) error {
		return f.PrintObject(cmd, false, o.Mapper, obj, out)
	}

	return nil
}

func (o *CreateImageStreamOptions) Validate() error {
	if o.IS == nil {
		return fmt.Errorf("IS is required")
	}
	if o.Client == nil {
		return fmt.Errorf("Client is required")
	}
	if o.Mapper == nil {
		return fmt.Errorf("Mapper is required")
	}
	if o.Out == nil {
		return fmt.Errorf("Out is required")
	}
	if o.Printer == nil {
		return fmt.Errorf("Printer is required")
	}

	return nil
}

func (o *CreateImageStreamOptions) Run() error {
	actualObj := o.IS

	var err error
	if !o.DryRun {
		actualObj, err = o.Client.ImageStreams(o.IS.Namespace).Create(o.IS)
		if err != nil {
			return err
		}
	}

	if useShortOutput := o.OutputFormat == "name"; useShortOutput || len(o.OutputFormat) == 0 {
		cmdutil.PrintSuccess(o.Mapper, useShortOutput, o.Out, "imagestream", actualObj.Name, o.DryRun, "created")
		return nil
	}

	return o.Printer(actualObj, o.Out)
}
