package create

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	utilenv "github.com/openshift/origin/pkg/oc/util/env"
)

const ImageStreamTagRecommendedName = "imagestreamtag"

var (
	imageStreamTagLong = templates.LongDesc(`
		Create a new image stream tag

		Image streams tags allow you to track, tag, and import images from other registries. They also 
		define an access controlled destination that you can push images to. An image stream tag can 
		reference images from many different registries and control how those images are referenced by 
		pods, deployments, and builds.

		If --resolve-local is passed, the image stream will be used as the source when pods reference
		it by name. For example, if stream 'mysql' resolves local names, a pod that points to
		'mysql:latest' will use the image the image stream points to under the "latest" tag.`)

	imageStreamTagExample = templates.Examples(`
		# Create a new image stream tag based on an image on a remote registry
		%[1]s mysql:latest --from-image=myregistry.local/mysql/mysql:5.0
		`)
)

type CreateImageStreamTagOptions struct {
	ISTag  *imageapi.ImageStreamTag
	Client imageclient.ImageStreamTagsGetter

	FromImage   string
	From        string
	Annotations []string

	DryRun bool

	Mapper       meta.RESTMapper
	OutputFormat string
	Out          io.Writer
	Printer      ObjectPrinter
}

// NewCmdCreateImageStreamTag is a command to create a new image stream tag.
func NewCmdCreateImageStreamTag(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &CreateImageStreamTagOptions{
		Out: out,
		ISTag: &imageapi.ImageStreamTag{
			ObjectMeta: metav1.ObjectMeta{},
			Tag:        &imageapi.TagReference{},
		},
	}

	cmd := &cobra.Command{
		Use:     name + " NAME",
		Short:   "Create a new image stream tag.",
		Long:    imageStreamTagLong,
		Example: fmt.Sprintf(imageStreamTagExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd, f, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
		Aliases: []string{"istag"},
	}

	cmd.Flags().StringVar(&o.FromImage, "from-image", "", "Use the provided remote image with this tag.")
	cmd.Flags().StringVar(&o.From, "from", "", "Use the provided image stream tag or image stream image as the source: [<namespace>/]name[:<tag>|@<id>]")
	cmd.Flags().StringSliceVarP(&o.Annotations, "annotation", "A", nil, "Set an annotation on this image stream tag.")
	cmd.Flags().BoolVar(&o.ISTag.Tag.ImportPolicy.Scheduled, "scheduled", false, "If set the remote source of this image will be periodically checked for imports.")
	cmd.Flags().BoolVar(&o.ISTag.Tag.ImportPolicy.Insecure, "insecure", false, "Allow importing from registries that are not fully secured by HTTPS.")
	cmd.Flags().StringVar((*string)(&o.ISTag.Tag.ReferencePolicy.Type), "reference-policy", (string)(o.ISTag.Tag.ReferencePolicy.Type), "If set to 'Local', referenced images will be pulled from the integrated registry. Ignored when reference is true.")
	cmd.Flags().BoolVar(&o.ISTag.Tag.Reference, "reference", o.ISTag.Tag.Reference, "If true, the tag value will be used whenever the image stream tag is referenced.")
	cmdutil.AddPrinterFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)
	return cmd
}

func (o *CreateImageStreamTagOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	o.DryRun = cmdutil.GetFlagBool(cmd, "dry-run")

	switch len(args) {
	case 0:
		return fmt.Errorf("image stream tag name is required")
	case 1:
		o.ISTag.Name = args[0]
	default:
		return fmt.Errorf("exactly one argument (name:tag) is supported, not: %v", args)
	}

	annotations, remove, err := utilenv.ParseAnnotation(o.Annotations, nil)
	if err != nil {
		return err
	}
	if len(remove) > 0 {
		return fmt.Errorf("annotations must be of the form name=value")
	}

	// to preserve backwards compatibility we are forced to set this
	o.ISTag.Annotations = annotations
	o.ISTag.Tag.Annotations = annotations

	o.ISTag.Namespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	client, err := f.OpenshiftInternalImageClient()
	if err != nil {
		return err
	}
	o.Client = client.Image()

	o.Mapper, _ = f.Object()
	o.OutputFormat = cmdutil.GetFlagString(cmd, "output")

	o.Printer = func(obj runtime.Object, out io.Writer) error {
		return f.PrintObject(cmd, false, o.Mapper, obj, out)
	}

	switch {
	case len(o.FromImage) > 0 && len(o.From) > 0:
		return fmt.Errorf("--from and --from-image may not be used together")
	case len(o.FromImage) > 0:
		o.ISTag.Tag.From = &kapi.ObjectReference{
			Name: o.FromImage,
			Kind: "DockerImage",
		}
	case len(o.From) > 0:
		var name string
		ref, err := imageapi.ParseDockerImageReference(o.From)
		if err != nil {
			if !strings.HasPrefix(o.From, ":") {
				return fmt.Errorf("Invalid --from, must be a valid image stream tag or image stream image: %v", err)
			}
			ref = imageapi.DockerImageReference{Tag: o.From[1:]}
			name = o.From[1:]
		} else {
			name = ref.NameString()
		}
		if len(ref.Registry) > 0 {
			return fmt.Errorf("Invalid --from, registry may not be specified")
		}
		kind := "ImageStreamTag"
		if len(ref.ID) > 0 {
			kind = "ImageStreamImage"
		}
		o.ISTag.Tag.From = &kapi.ObjectReference{
			Kind:      kind,
			Name:      name,
			Namespace: ref.Namespace,
		}
	}

	return nil
}

func (o *CreateImageStreamTagOptions) Validate() error {
	if o.ISTag == nil {
		return fmt.Errorf("ISTag is required")
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

func (o *CreateImageStreamTagOptions) Run() error {
	actualObj := o.ISTag

	var err error
	if !o.DryRun {
		actualObj, err = o.Client.ImageStreamTags(o.ISTag.Namespace).Create(o.ISTag)
		if err != nil {
			return err
		}
	}

	if useShortOutput := o.OutputFormat == "name"; useShortOutput || len(o.OutputFormat) == 0 {
		cmdutil.PrintSuccess(o.Mapper, useShortOutput, o.Out, "imagestreamtag", actualObj.Name, o.DryRun, "created")
		return nil
	}

	return o.Printer(actualObj, o.Out)
}
