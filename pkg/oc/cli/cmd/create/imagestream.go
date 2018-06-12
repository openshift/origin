package create

import (
	"fmt"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	imagev1 "github.com/openshift/api/image/v1"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
)

const ImageStreamRecommendedName = "imagestream"

var (
	imageStreamLong = templates.LongDesc(`
		Create a new image stream

		Image streams allow you to track, tag, and import images from other registries. They also define an
		access controlled destination that you can push images to. An image stream can reference images
		from many different registries and control how those images are referenced by pods, deployments,
		and builds.

		If --lookup-local is passed, the image stream will be used as the source when pods reference
		it by name. For example, if stream 'mysql' resolves local names, a pod that points to
		'mysql:latest' will use the image the image stream points to under the "latest" tag.`)

	imageStreamExample = templates.Examples(`
		# Create a new image stream
  	%[1]s mysql`)
)

type CreateImageStreamOptions struct {
	CreateSubcommandOptions *CreateSubcommandOptions

	LookupLocal bool

	Client imagev1client.ImageStreamsGetter
}

// NewCmdCreateImageStream is a macro command to create a new image stream
func NewCmdCreateImageStream(name, fullName string, f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateImageStreamOptions{
		CreateSubcommandOptions: NewCreateSubcommandOptions(streams),
	}
	cmd := &cobra.Command{
		Use:     name + " NAME",
		Short:   "Create a new empty image stream.",
		Long:    imageStreamLong,
		Example: fmt.Sprintf(imageStreamExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd, f, args))
			cmdutil.CheckErr(o.Run())
		},
		Aliases: []string{"is"},
	}
	cmd.Flags().BoolVar(&o.LookupLocal, "lookup-local", o.LookupLocal, "If true, the image stream will be the source for any top-level image reference in this project.")

	o.CreateSubcommandOptions.PrintFlags.AddFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *CreateImageStreamOptions) Complete(cmd *cobra.Command, f genericclioptions.RESTClientGetter, args []string) error {
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.Client, err = imagev1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	return o.CreateSubcommandOptions.Complete(f, cmd, args)
}

func (o *CreateImageStreamOptions) Run() error {
	imageStream := &imagev1.ImageStream{
		// this is ok because we know exactly how we want to be serialized
		TypeMeta:   metav1.TypeMeta{APIVersion: imagev1.SchemeGroupVersion.String(), Kind: "ImageStream"},
		ObjectMeta: metav1.ObjectMeta{Name: o.CreateSubcommandOptions.Name},
		Spec: imagev1.ImageStreamSpec{
			LookupPolicy: imagev1.ImageLookupPolicy{
				Local: o.LookupLocal,
			},
		},
	}

	if !o.CreateSubcommandOptions.DryRun {
		var err error
		imageStream, err = o.Client.ImageStreams(o.CreateSubcommandOptions.Namespace).Create(imageStream)
		if err != nil {
			return err
		}
	}

	return o.CreateSubcommandOptions.Printer.PrintObj(imageStream, o.CreateSubcommandOptions.Out)
}
