package create

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	imagev1 "github.com/openshift/api/image/v1"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
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
	CreateSubcommandOptions *CreateSubcommandOptions

	Client imagev1client.ImageStreamTagsGetter

	FromImage          string
	From               string
	Annotations        []string
	Scheduled          bool
	Insecure           bool
	Reference          bool
	ReferencePolicyStr string
	ReferencePolicy    imagev1.TagReferencePolicyType
}

// NewCmdCreateImageStreamTag is a command to create a new image stream tag.
func NewCmdCreateImageStreamTag(name, fullName string, f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateImageStreamTagOptions{
		CreateSubcommandOptions: NewCreateSubcommandOptions(streams),
	}
	cmd := &cobra.Command{
		Use:     name + " NAME",
		Short:   "Create a new image stream tag.",
		Long:    imageStreamTagLong,
		Example: fmt.Sprintf(imageStreamTagExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd, f, args))
			cmdutil.CheckErr(o.Run())
		},
		Aliases: []string{"istag"},
	}
	cmd.Flags().StringVar(&o.FromImage, "from-image", o.FromImage, "Use the provided remote image with this tag.")
	cmd.Flags().StringVar(&o.From, "from", o.From, "Use the provided image stream tag or image stream image as the source: [<namespace>/]name[:<tag>|@<id>]")
	cmd.Flags().StringSliceVarP(&o.Annotations, "annotation", "A", o.Annotations, "Set an annotation on this image stream tag.")
	cmd.Flags().MarkShorthandDeprecated("annotation", "please use --annotation instead.")
	cmd.Flags().BoolVar(&o.Scheduled, "scheduled", o.Scheduled, "If set the remote source of this image will be periodically checked for imports.")
	cmd.Flags().BoolVar(&o.Insecure, "insecure", o.Insecure, "Allow importing from registries that are not fully secured by HTTPS.")
	cmd.Flags().StringVar(&o.ReferencePolicyStr, "reference-policy", o.ReferencePolicyStr, "If set to 'Local', referenced images will be pulled from the integrated registry. Ignored when reference is true.")
	cmd.Flags().BoolVar(&o.Reference, "reference", o.Reference, "If true, the tag value will be used whenever the image stream tag is referenced.")

	o.CreateSubcommandOptions.PrintFlags.AddFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *CreateImageStreamTagOptions) Complete(cmd *cobra.Command, f genericclioptions.RESTClientGetter, args []string) error {
	if len(o.ReferencePolicyStr) > 0 {
		switch strings.ToLower(o.ReferencePolicyStr) {
		case "source":
			o.ReferencePolicy = imagev1.SourceTagReferencePolicy
		case "local":
			o.ReferencePolicy = imagev1.LocalTagReferencePolicy
		default:
			return fmt.Errorf("valid values for --reference-policy are: source, local")
		}
	}

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

func (o *CreateImageStreamTagOptions) Run() error {
	isTag := &imagev1.ImageStreamTag{
		// this is ok because we know exactly how we want to be serialized
		TypeMeta: metav1.TypeMeta{APIVersion: imagev1.SchemeGroupVersion.String(), Kind: "ImageStreamTag"},
		ObjectMeta: metav1.ObjectMeta{
			Name: o.CreateSubcommandOptions.Name,
		},
		Tag: &imagev1.TagReference{
			ImportPolicy: imagev1.TagImportPolicy{
				Scheduled: o.Scheduled,
				Insecure:  o.Insecure,
			},
			ReferencePolicy: imagev1.TagReferencePolicy{
				Type: o.ReferencePolicy,
			},
			Reference: o.Reference,
		},
	}

	annotations, remove, err := utilenv.ParseAnnotation(o.Annotations, nil)
	if err != nil {
		return err
	}
	if len(remove) > 0 {
		return fmt.Errorf("annotations must be of the form name=value")
	}

	// to preserve backwards compatibility we are forced to set this
	isTag.Annotations = annotations
	isTag.Tag.Annotations = annotations

	switch {
	case len(o.FromImage) > 0 && len(o.From) > 0:
		return fmt.Errorf("--from and --from-image may not be used together")
	case len(o.FromImage) > 0:
		isTag.Tag.From = &corev1.ObjectReference{
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
		isTag.Tag.From = &corev1.ObjectReference{
			Kind:      kind,
			Name:      name,
			Namespace: ref.Namespace,
		}
	}

	if !o.CreateSubcommandOptions.DryRun {
		isTag, err = o.Client.ImageStreamTags(o.CreateSubcommandOptions.Namespace).Create(isTag)
		if err != nil {
			return err
		}
	}

	return o.CreateSubcommandOptions.Printer.PrintObj(isTag, o.CreateSubcommandOptions.Out)
}
