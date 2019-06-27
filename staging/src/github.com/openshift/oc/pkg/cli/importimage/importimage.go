package importimage

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	imagev1 "github.com/openshift/api/image/v1"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	"github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/oc/pkg/cli/tag"
	"github.com/openshift/oc/pkg/helpers/describe"
	imagehelpers "github.com/openshift/oc/pkg/helpers/image"
)

var (
	importImageLong = templates.LongDesc(`
		Import the latest image information from a tag in a container image registry

		Image streams allow you to control which images are rolled out to your builds
		and applications. This command fetches the latest version of an image from a
		remote repository and updates the image stream tag if it does not match the
		previous value. Running the command multiple times will not create duplicate
		entries. When importing an image, only the image metadata is copied, not the
		image contents.

		If you wish to change the image stream tag or provide more advanced options,
		see the 'tag' command.`)

	importImageExample = templates.Examples(`
	  %[1]s import-image mystream
		`)
)

// ImageImportOptions contains all the necessary information to perform an import.
type ImportImageOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	ToPrinter func(string) (printers.ResourcePrinter, error)

	// user set values
	From                 string
	Confirm              bool
	All                  bool
	Scheduled            bool
	Insecure             bool
	InsecureFlagProvided bool

	DryRun bool

	// internal values
	Namespace       string
	Name            string
	Tag             string
	Target          string
	ReferencePolicy string

	// helpers
	imageClient imagev1client.ImageV1Interface
	isClient    imagev1client.ImageStreamInterface

	genericclioptions.IOStreams
}

func NewImportImageOptions(name string, streams genericclioptions.IOStreams) *ImportImageOptions {
	return &ImportImageOptions{
		PrintFlags:      genericclioptions.NewPrintFlags("imported"),
		IOStreams:       streams,
		ReferencePolicy: tag.SourceReferencePolicy,
	}
}

// NewCmdImportImage implements the OpenShift cli import-image command.
func NewCmdImportImage(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewImportImageOptions(fullName, streams)

	cmd := &cobra.Command{
		Use:     "import-image IMAGESTREAM[:TAG]",
		Short:   "Imports images from a container image registry",
		Long:    importImageLong,
		Example: fmt.Sprintf(importImageExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}

	o.PrintFlags.AddFlags(cmd)

	cmd.Flags().StringVar(&o.From, "from", o.From, "A Docker image repository to import images from")
	cmd.Flags().BoolVar(&o.Confirm, "confirm", o.Confirm, "If true, allow the image stream import location to be set or changed")
	cmd.Flags().BoolVar(&o.All, "all", o.All, "If true, import all tags from the provided source on creation or if --from is specified")
	cmd.Flags().StringVar(&o.ReferencePolicy, "reference-policy", o.ReferencePolicy, "Allow to request pullthrough for external image when set to 'local'. Defaults to 'source'.")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", o.DryRun, "Fetch information about images without creating or updating an image stream.")
	cmd.Flags().BoolVar(&o.Scheduled, "scheduled", o.Scheduled, "Set each imported Docker image to be periodically imported from a remote repository. Defaults to false.")
	cmd.Flags().BoolVar(&o.Insecure, "insecure", o.Insecure, "If true, allow importing from registries that have invalid HTTPS certificates or are hosted via HTTP. This flag will take precedence over the insecure annotation.")

	return cmd
}

// Complete turns a partially defined ImportImageOptions into a solvent structure
// which can be validated and used for aa import.
func (o *ImportImageOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		o.Target = args[0]
	}

	o.InsecureFlagProvided = cmd.Flags().Lookup("insecure").Changed
	if !cmd.Flags().Lookup("reference-policy").Changed {
		o.ReferencePolicy = ""
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.imageClient, err = imagev1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.isClient = o.imageClient.ImageStreams(o.Namespace)

	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation

		// We assume that the (dry run) message has already been added (if need be)
		// by the caller of this method.
		return o.PrintFlags.ToPrinter()
	}

	return o.parseImageReference()
}

func (o *ImportImageOptions) parseImageReference() error {
	targetRef, err := reference.Parse(o.Target)
	switch {
	case err != nil:
		return fmt.Errorf("the image name must be a valid Docker image pull spec or reference to an image stream (e.g. myregistry/myteam/image:tag)")
	case len(targetRef.ID) > 0:
		return fmt.Errorf("to import images by ID, use the 'tag' command")
	case len(targetRef.Tag) != 0 && o.All:
		// error out
		return fmt.Errorf("cannot specify a tag %q as well as --all", o.Target)
	case len(targetRef.Tag) == 0 && !o.All:
		// apply the default tag
		targetRef.Tag = imagev1.DefaultImageTag
	}
	o.Name = targetRef.Name
	o.Tag = targetRef.Tag

	return nil
}

// Validate ensures that a ImportImageOptions is valid and can be used to execute
// an import.
func (o *ImportImageOptions) Validate() error {
	if len(o.Target) == 0 {
		return fmt.Errorf("you must specify the name of an image stream")
	}

	return nil
}

// Run contains all the necessary functionality for the OpenShift cli import-image command.
func (o *ImportImageOptions) Run() error {
	stream, isi, err := o.createImageImport()
	if err != nil {
		return err
	}

	result, err := o.imageClient.ImageStreamImports(isi.Namespace).Create(isi)
	if err != nil {
		return err
	}

	message := "imported"
	if wasError(result) {
		message = "imported with errors"
	}

	if o.DryRun {
		message = fmt.Sprintf("%s (dry run)", message)
	}
	message = fmt.Sprintf("%s\n\n", message)

	if result.Status.Import != nil {
		// TODO: dry-run doesn't return an image stream, so we have to display partial results
		info, err := describe.DescribeImageStream(result.Status.Import)
		if err != nil {
			return err
		}

		message += fmt.Sprintln(info)
	}

	if repo := result.Status.Repository; repo != nil {
		for _, image := range repo.Images {
			if image.Image != nil {
				info, err := describe.DescribeImage(image.Image, imageutil.JoinImageStreamTag(stream.Name, image.Tag))
				if err != nil {
					fmt.Fprintf(o.ErrOut, "error: tag %s failed: %v\n", image.Tag, err)
				} else {
					message += fmt.Sprintln(info)
				}
			} else {
				fmt.Fprintf(o.ErrOut, "error: repository tag %s failed: %v\n", image.Tag, image.Status.Message)
			}
		}
	}

	for _, image := range result.Status.Images {
		if image.Image != nil {
			info, err := describe.DescribeImage(image.Image, imageutil.JoinImageStreamTag(stream.Name, image.Tag))
			if err != nil {
				fmt.Fprintf(o.ErrOut, "error: tag %s failed: %v\n", image.Tag, err)
			} else {
				message += fmt.Sprintln(info)
			}
		} else {
			fmt.Fprintf(o.ErrOut, "error: tag %s failed: %v\n", image.Tag, image.Status.Message)
		}
	}

	if r := result.Status.Repository; r != nil && len(r.AdditionalTags) > 0 {
		message += fmt.Sprintf("\ninfo: The remote repository contained %d additional tags which were not imported: %s\n", len(r.AdditionalTags), strings.Join(r.AdditionalTags, ", "))
	}

	printer, err := o.ToPrinter(message)
	if err != nil {
		return err
	}

	return printer.PrintObj(stream, o.Out)
}

func wasError(isi *imagev1.ImageStreamImport) bool {
	for _, image := range isi.Status.Images {
		if image.Status.Status == metav1.StatusFailure {
			return true
		}
	}
	if isi.Status.Repository != nil && isi.Status.Repository.Status.Status == metav1.StatusFailure {
		return true
	}
	return false
}

// TODO: move to image/api as a helper
type importError struct {
	annotation string
}

func (e importError) Error() string {
	return fmt.Sprintf("unable to import image: %s", e.annotation)
}

func (o *ImportImageOptions) createImageImport() (*imagev1.ImageStream, *imagev1.ImageStreamImport, error) {
	var isi *imagev1.ImageStreamImport
	stream, err := o.isClient.Get(o.Name, metav1.GetOptions{})
	// no stream, try creating one
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, nil, err
		}
		if !o.Confirm {
			return nil, nil, fmt.Errorf("no image stream named %q exists, pass --confirm to create and import", o.Name)
		}
		stream, isi = o.newImageStream()
		return stream, isi, nil
	}

	if o.All {
		// importing the entire repository
		isi, err = o.importAll(stream)
		if err != nil {
			return nil, nil, err
		}
	} else {
		// importing a single tag
		isi, err = o.importTag(stream)
		if err != nil {
			return nil, nil, err
		}
	}

	// this is ok because we know exactly how we want to be serialized
	if stream.GetObjectKind().GroupVersionKind().Empty() {
		stream.GetObjectKind().SetGroupVersionKind(imagev1.SchemeGroupVersion.WithKind("ImageStream"))
	}

	return stream, isi, nil
}

func (o *ImportImageOptions) importAll(stream *imagev1.ImageStream) (*imagev1.ImageStreamImport, error) {
	from := o.From
	// update ImageStream appropriately
	if len(from) == 0 {
		if len(stream.Spec.DockerImageRepository) != 0 {
			from = stream.Spec.DockerImageRepository
		} else {
			tags := make(map[string]string)
			for _, tag := range stream.Spec.Tags {
				if tag.From != nil && tag.From.Kind == "DockerImage" {
					tags[tag.Name] = tag.From.Name
				}
			}
			if len(tags) == 0 {
				return nil, fmt.Errorf("image stream does not have tags pointing to external container images")
			}
			return o.newImageStreamImportTags(stream, tags), nil
		}
	}
	if from != stream.Spec.DockerImageRepository {
		if !o.Confirm {
			if len(stream.Spec.DockerImageRepository) == 0 {
				return nil, fmt.Errorf("the image stream does not currently import an entire Docker repository, pass --confirm to update")
			}
			return nil, fmt.Errorf("the image stream has a different import spec %q, pass --confirm to update", stream.Spec.DockerImageRepository)
		}
		stream.Spec.DockerImageRepository = from
	}

	// and create accompanying ImageStreamImport
	return o.newImageStreamImportAll(stream, from), nil
}

func (o *ImportImageOptions) importTag(stream *imagev1.ImageStream) (*imagev1.ImageStreamImport, error) {
	from := o.From
	tag := o.Tag

	// follow any referential tags to the destination
	finalTag, existing, multiple, err := imagehelpers.FollowTagReference(stream, tag)
	switch err {
	case imagehelpers.ErrInvalidReference:
		return nil, fmt.Errorf("tag %q points to an invalid imagestreamtag", tag)
	case imagehelpers.ErrCrossImageStreamReference:
		return nil, fmt.Errorf("tag %q points to an imagestreamtag from another ImageStream", tag)
	case imagehelpers.ErrCircularReference:
		return nil, fmt.Errorf("tag %q on the image stream is a reference to same tag", tag)
	case imagehelpers.ErrNotFoundReference:
		// create a new tag
		if len(from) == 0 && tag == imagev1.DefaultImageTag {
			from = stream.Spec.DockerImageRepository
		}
		// if the from is still empty this means there's no such tag defined
		// nor we can't create any from .spec.dockerImageRepository
		if len(from) == 0 {
			return nil, fmt.Errorf("the tag %q does not exist on the image stream - choose an existing tag to import or use the 'tag' command to create a new tag", tag)
		}
		existing = &imagev1.TagReference{
			From: &corev1.ObjectReference{
				Kind: "DockerImage",
				Name: from,
			},
		}
	case nil:
		// disallow re-importing anything other than DockerImage
		if existing.From != nil && existing.From.Kind != "DockerImage" {
			return nil, fmt.Errorf("tag %q points to existing %s %q, it cannot be re-imported", tag, existing.From.Kind, existing.From.Name)
		}
		// disallow changing an existing tag
		if existing.From == nil {
			return nil, fmt.Errorf("tag %q already exists - you must use the 'tag' command if you want to change the source to %q", tag, from)
		}
		if len(from) != 0 && from != existing.From.Name {
			if multiple {
				return nil, fmt.Errorf("the tag %q points to the tag %q which points to %q - use the 'tag' command if you want to change the source to %q",
					tag, finalTag, existing.From.Name, from)
			}
			return nil, fmt.Errorf("the tag %q points to %q - use the 'tag' command if you want to change the source to %q", tag, existing.From.Name, from)
		}

		// set the target item to import
		from = existing.From.Name
		if multiple {
			tag = finalTag
		}

		// clear the legacy annotation
		delete(existing.Annotations, imagev1.DockerImageRepositoryCheckAnnotation)
		// reset the generation
		zero := int64(0)
		existing.Generation = &zero

	}

	tagFound := false
	for i := range stream.Spec.Tags {
		if stream.Spec.Tags[i].Name == tag {
			stream.Spec.Tags[i] = *existing
			tagFound = true
			break
		}
	}

	if !tagFound {
		stream.Spec.Tags = append(stream.Spec.Tags, *existing)
	}

	// and create accompanying ImageStreamImport
	return o.newImageStreamImportTags(stream, map[string]string{tag: from}), nil
}

func (o *ImportImageOptions) newImageStream() (*imagev1.ImageStream, *imagev1.ImageStreamImport) {
	from := o.From
	tag := o.Tag
	if len(from) == 0 {
		from = o.Target
	}
	var (
		stream *imagev1.ImageStream
		isi    *imagev1.ImageStreamImport
	)
	// create new ImageStream and accompanying ImageStreamImport
	// TODO: this should be removed along with the legacy path, we don't need to
	// create the IS in the new path, the import mechanism will do that for us,
	// this is only for the legacy path that we need to create the IS.
	if o.All {
		stream = &imagev1.ImageStream{
			// this is ok because we know exactly how we want to be serialized
			TypeMeta:   metav1.TypeMeta{APIVersion: imagev1.SchemeGroupVersion.String(), Kind: "ImageStream"},
			ObjectMeta: metav1.ObjectMeta{Name: o.Name},
			Spec:       imagev1.ImageStreamSpec{DockerImageRepository: from},
		}
		isi = o.newImageStreamImportAll(stream, from)
	} else {
		stream = &imagev1.ImageStream{
			// this is ok because we know exactly how we want to be serialized
			TypeMeta:   metav1.TypeMeta{APIVersion: imagev1.SchemeGroupVersion.String(), Kind: "ImageStream"},
			ObjectMeta: metav1.ObjectMeta{Name: o.Name},
			Spec: imagev1.ImageStreamSpec{
				Tags: []imagev1.TagReference{
					{
						From: &corev1.ObjectReference{
							Kind: "DockerImage",
							Name: from,
						},
						ReferencePolicy: o.getReferencePolicy(),
					},
				},
			},
		}
		isi = o.newImageStreamImportTags(stream, map[string]string{tag: from})
	}

	return stream, isi
}

func (o *ImportImageOptions) getReferencePolicy() imagev1.TagReferencePolicy {
	ref := imagev1.TagReferencePolicy{}
	if len(o.ReferencePolicy) == 0 {
		return ref
	}
	switch o.ReferencePolicy {
	case tag.SourceReferencePolicy:
		ref.Type = imagev1.SourceTagReferencePolicy
	case tag.LocalReferencePolicy:
		ref.Type = imagev1.LocalTagReferencePolicy
	}
	return ref
}

func (o *ImportImageOptions) newImageStreamImport(stream *imagev1.ImageStream) (*imagev1.ImageStreamImport, bool) {
	isi := &imagev1.ImageStreamImport{
		ObjectMeta: metav1.ObjectMeta{
			Name:            stream.Name,
			Namespace:       o.Namespace,
			ResourceVersion: stream.ResourceVersion,
		},
		Spec: imagev1.ImageStreamImportSpec{Import: !o.DryRun},
	}
	insecureAnnotation := stream.Annotations[imagev1.InsecureRepositoryAnnotation]
	insecure := insecureAnnotation == "true"
	// --insecure flag (if provided) takes precedence over insecure annotation
	if o.InsecureFlagProvided {
		insecure = o.Insecure
	}

	return isi, insecure
}

func (o *ImportImageOptions) newImageStreamImportAll(stream *imagev1.ImageStream, from string) *imagev1.ImageStreamImport {
	isi, insecure := o.newImageStreamImport(stream)
	isi.Spec.Repository = &imagev1.RepositoryImportSpec{
		From: corev1.ObjectReference{
			Kind: "DockerImage",
			Name: from,
		},
		ImportPolicy: imagev1.TagImportPolicy{
			Insecure:  insecure,
			Scheduled: o.Scheduled,
		},
		ReferencePolicy: o.getReferencePolicy(),
	}

	return isi
}

func (o *ImportImageOptions) newImageStreamImportTags(stream *imagev1.ImageStream, tags map[string]string) *imagev1.ImageStreamImport {
	isi, streamInsecure := o.newImageStreamImport(stream)
	for tag, from := range tags {
		insecure := streamInsecure
		scheduled := o.Scheduled

		oldTagFound := false
		var oldTag imagev1.TagReference
		for _, t := range stream.Spec.Tags {
			if t.Name == tag {
				oldTag = t
				oldTagFound = true
				break
			}
		}

		if oldTagFound {
			insecure = insecure || oldTag.ImportPolicy.Insecure
			scheduled = scheduled || oldTag.ImportPolicy.Scheduled
		}
		isi.Spec.Images = append(isi.Spec.Images, imagev1.ImageImportSpec{
			From: corev1.ObjectReference{
				Kind: "DockerImage",
				Name: from,
			},
			To: &corev1.LocalObjectReference{Name: tag},
			ImportPolicy: imagev1.TagImportPolicy{
				Insecure:  insecure,
				Scheduled: scheduled,
			},
			ReferencePolicy: o.getReferencePolicy(),
		})
	}
	return isi
}
