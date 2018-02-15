package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kprinters "k8s.io/kubernetes/pkg/printers"

	imageapiv1 "github.com/openshift/api/image/v1"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
	"github.com/openshift/origin/pkg/oc/cli/describe"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

var (
	importImageLong = templates.LongDesc(`
		Import the latest image information from a tag in a Docker registry

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

// NewCmdImportImage implements the OpenShift cli import-image command.
func NewCmdImportImage(fullName string, f *clientcmd.Factory, out, errout io.Writer) *cobra.Command {
	opts := &ImportImageOptions{}
	cmd := &cobra.Command{
		Use:     "import-image IMAGESTREAM[:TAG]",
		Short:   "Imports images from a Docker registry",
		Long:    importImageLong,
		Example: fmt.Sprintf(importImageExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, cmd, args, fullName, out, errout))
			kcmdutil.CheckErr(opts.Validate(cmd))
			kcmdutil.CheckErr(opts.Run())
		},
	}
	cmd.Flags().StringVar(&opts.From, "from", "", "A Docker image repository to import images from")
	cmd.Flags().BoolVar(&opts.Confirm, "confirm", false, "If true, allow the image stream import location to be set or changed")
	cmd.Flags().BoolVar(&opts.All, "all", false, "If true, import all tags from the provided source on creation or if --from is specified")
	cmd.Flags().StringVar(&opts.ReferencePolicy, "reference-policy", sourceReferencePolicy, "Allow to request pullthrough for external image when set to 'local'. Defaults to 'source'.")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Fetch information about images without creating or updating an image stream.")
	cmd.Flags().BoolVar(&opts.Scheduled, "scheduled", false, "Set each imported Docker image to be periodically imported from a remote repository. Defaults to false.")
	opts.Insecure = cmd.Flags().Bool("insecure", false, "If true, allow importing from registries that have invalid HTTPS certificates or are hosted via HTTP. This flag will take precedence over the insecure annotation.")

	return cmd
}

// ImageImportOptions contains all the necessary information to perform an import.
type ImportImageOptions struct {
	// user set values
	From      string
	Confirm   bool
	All       bool
	Scheduled bool
	Insecure  *bool

	DryRun bool

	// internal values
	Namespace       string
	Name            string
	Tag             string
	Target          string
	ReferencePolicy string

	CommandName string

	// helpers
	out         io.Writer
	errout      io.Writer
	imageClient imageclient.ImageInterface
	isClient    imageclient.ImageStreamInterface
}

// Complete turns a partially defined ImportImageOptions into a solvent structure
// which can be validated and used for aa import.
func (o *ImportImageOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, commandName string, out, errout io.Writer) error {
	o.CommandName = commandName

	if len(args) > 0 {
		o.Target = args[0]
	}

	if !cmd.Flags().Lookup("insecure").Changed {
		o.Insecure = nil
	}
	if !cmd.Flags().Lookup("reference-policy").Changed {
		o.ReferencePolicy = ""
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace

	client, err := f.OpenshiftInternalImageClient()
	if err != nil {
		return err
	}
	o.imageClient = client.Image()
	o.isClient = client.Image().ImageStreams(namespace)

	o.out = out
	o.errout = errout

	return nil
}

// Validate ensures that a ImportImageOptions is valid and can be used to execute
// an import.
func (o *ImportImageOptions) Validate(cmd *cobra.Command) error {
	if len(o.Target) == 0 {
		return kcmdutil.UsageErrorf(cmd, "you must specify the name of an image stream")
	}

	targetRef, err := imageapi.ParseDockerImageReference(o.Target)
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
		targetRef.Tag = imageapi.DefaultImageTag
	}
	o.Name = targetRef.Name
	o.Tag = targetRef.Tag

	return nil
}

// Run contains all the necessary functionality for the OpenShift cli import-image command.
func (o *ImportImageOptions) Run() error {
	stream, isi, err := o.createImageImport()
	if err != nil {
		return err
	}

	// Attempt the new, direct import path
	result, err := o.imageClient.ImageStreamImports(isi.Namespace).Create(isi)
	err = TransformUnsupportedError(err)
	switch {
	case err == imageapi.ErrImageStreamImportUnsupported:
	case err != nil:
		return err
	default:
		if o.DryRun {
			if wasError(result) {
				fmt.Fprintf(o.errout, "The dry-run import completed with errors.\n\n")
			} else {
				fmt.Fprint(o.out, "The dry-run import completed successfully.\n\n")
			}
		} else {
			if wasError(result) {
				fmt.Fprintf(o.errout, "The import completed with errors.\n\n")
			} else {
				fmt.Fprint(o.out, "The import completed successfully.\n\n")
			}
		}

		if result.Status.Import != nil {
			// TODO: dry-run doesn't return an image stream, so we have to display partial results
			info, err := describe.DescribeImageStream(result.Status.Import)
			if err != nil {
				return err
			}
			fmt.Fprintln(o.out, info)
		}

		if repo := result.Status.Repository; repo != nil {
			for _, image := range repo.Images {
				if image.Image != nil {
					info, err := describe.DescribeImage(image.Image, imageapi.JoinImageStreamTag(stream.Name, image.Tag))
					if err != nil {
						fmt.Fprintf(o.errout, "error: tag %s failed: %v\n", image.Tag, err)
					} else {
						fmt.Fprintln(o.out, info)
					}
				} else {
					fmt.Fprintf(o.errout, "error: repository tag %s failed: %v\n", image.Tag, image.Status.Message)
				}
			}
		}

		for _, image := range result.Status.Images {
			if image.Image != nil {
				info, err := describe.DescribeImage(image.Image, imageapi.JoinImageStreamTag(stream.Name, image.Tag))
				if err != nil {
					fmt.Fprintf(o.errout, "error: tag %s failed: %v\n", image.Tag, err)
				} else {
					fmt.Fprintln(o.out, info)
				}
			} else {
				fmt.Fprintf(o.errout, "error: tag %s failed: %v\n", image.Tag, image.Status.Message)
			}
		}

		if r := result.Status.Repository; r != nil && len(r.AdditionalTags) > 0 {
			fmt.Fprintf(o.out, "\ninfo: The remote repository contained %d additional tags which were not imported: %s\n", len(r.AdditionalTags), strings.Join(r.AdditionalTags, ", "))
		}
		return nil
	}

	// Legacy path, remove when support for older importers is removed
	delete(stream.Annotations, imageapi.DockerImageRepositoryCheckAnnotation)
	if o.Insecure != nil && *o.Insecure {
		if stream.Annotations == nil {
			stream.Annotations = make(map[string]string)
		}
		stream.Annotations[imageapi.InsecureRepositoryAnnotation] = "true"
	}

	if stream.CreationTimestamp.IsZero() {
		stream, err = o.isClient.Create(stream)
	} else {
		stream, err = o.isClient.Update(stream)
	}
	if err != nil {
		return err
	}

	fmt.Fprintln(o.out, "Importing (ctrl+c to stop waiting) ...")

	resourceVersion := stream.ResourceVersion
	updatedStream, err := o.waitForImport(resourceVersion)
	if err != nil {
		if _, ok := err.(importError); ok {
			return err
		}
		return fmt.Errorf("unable to determine if the import completed successfully - please run '%s describe -n %s imagestream/%s' to see if the tags were updated as expected: %v", o.CommandName, stream.Namespace, stream.Name, err)
	}

	fmt.Fprint(o.out, "The import completed successfully.\n\n")

	d := describe.ImageStreamDescriber{ImageClient: o.imageClient}
	info, err := d.Describe(updatedStream.Namespace, updatedStream.Name, kprinters.DescriberSettings{})
	if err != nil {
		return err
	}

	fmt.Fprintln(o.out, info)
	return nil
}

func wasError(isi *imageapi.ImageStreamImport) bool {
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

func (o *ImportImageOptions) waitForImport(resourceVersion string) (*imageapi.ImageStream, error) {
	streamWatch, err := o.isClient.Watch(metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector("metadata.name", o.Name).String(), ResourceVersion: resourceVersion})
	if err != nil {
		return nil, err
	}
	defer streamWatch.Stop()

	for {
		select {
		case event, ok := <-streamWatch.ResultChan():
			if !ok {
				return nil, fmt.Errorf("image stream watch ended prematurely")
			}

			switch event.Type {
			case watch.Modified:
				s, ok := event.Object.(*imageapi.ImageStream)
				if !ok {
					continue
				}
				annotation, ok := s.Annotations[imageapi.DockerImageRepositoryCheckAnnotation]
				if !ok {
					continue
				}

				if _, err := time.Parse(time.RFC3339, annotation); err == nil {
					return s, nil
				}
				return nil, importError{annotation}

			case watch.Deleted:
				return nil, fmt.Errorf("the image stream was deleted")
			case watch.Error:
				return nil, fmt.Errorf("error watching image stream")
			}
		}
	}
}

func (o *ImportImageOptions) createImageImport() (*imageapi.ImageStream, *imageapi.ImageStreamImport, error) {
	var isi *imageapi.ImageStreamImport
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
		// ensure defaulting is applied by round trip converting
		// TODO: convert to using versioned types.
		external, err := legacyscheme.Scheme.ConvertToVersion(stream, imageapiv1.SchemeGroupVersion)
		if err != nil {
			return nil, nil, err
		}
		legacyscheme.Scheme.Default(external)
		internal, err := legacyscheme.Scheme.ConvertToVersion(external, imageapi.SchemeGroupVersion)
		if err != nil {
			return nil, nil, err
		}
		stream = internal.(*imageapi.ImageStream)
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

	return stream, isi, nil
}

func (o *ImportImageOptions) importAll(stream *imageapi.ImageStream) (*imageapi.ImageStreamImport, error) {
	from := o.From
	// update ImageStream appropriately
	if len(from) == 0 {
		if len(stream.Spec.DockerImageRepository) != 0 {
			from = stream.Spec.DockerImageRepository
		} else {
			tags := make(map[string]string)
			for name, tag := range stream.Spec.Tags {
				if tag.From != nil && tag.From.Kind == "DockerImage" {
					tags[name] = tag.From.Name
				}
			}
			if len(tags) == 0 {
				return nil, fmt.Errorf("image stream does not have tags pointing to external docker images")
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

func (o *ImportImageOptions) importTag(stream *imageapi.ImageStream) (*imageapi.ImageStreamImport, error) {
	from := o.From
	tag := o.Tag
	// follow any referential tags to the destination
	finalTag, existing, multiple, err := imageapi.FollowTagReference(stream, tag)
	switch err {
	case imageapi.ErrInvalidReference:
		return nil, fmt.Errorf("tag %q points to an invalid imagestreamtag", tag)
	case imageapi.ErrCrossImageStreamReference:
		return nil, fmt.Errorf("tag %q points to an imagestreamtag from another ImageStream", tag)
	case imageapi.ErrCircularReference:
		return nil, fmt.Errorf("tag %q on the image stream is a reference to same tag", tag)
	case imageapi.ErrNotFoundReference:
		// create a new tag
		if len(from) == 0 && tag == imageapi.DefaultImageTag {
			from = stream.Spec.DockerImageRepository
		}
		// if the from is still empty this means there's no such tag defined
		// nor we can't create any from .spec.dockerImageRepository
		if len(from) == 0 {
			return nil, fmt.Errorf("the tag %q does not exist on the image stream - choose an existing tag to import or use the 'tag' command to create a new tag", tag)
		}
		existing = &imageapi.TagReference{
			From: &kapi.ObjectReference{
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
		delete(existing.Annotations, imageapi.DockerImageRepositoryCheckAnnotation)
		// reset the generation
		zero := int64(0)
		existing.Generation = &zero

	}
	stream.Spec.Tags[tag] = *existing

	// and create accompanying ImageStreamImport
	return o.newImageStreamImportTags(stream, map[string]string{tag: from}), nil
}

func (o *ImportImageOptions) newImageStream() (*imageapi.ImageStream, *imageapi.ImageStreamImport) {
	from := o.From
	tag := o.Tag
	if len(from) == 0 {
		from = o.Target
	}
	var (
		stream *imageapi.ImageStream
		isi    *imageapi.ImageStreamImport
	)
	// create new ImageStream and accompanying ImageStreamImport
	// TODO: this should be removed along with the legacy path, we don't need to
	// create the IS in the new path, the import mechanism will do that for us,
	// this is only for the legacy path that we need to create the IS.
	if o.All {
		stream = &imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{Name: o.Name},
			Spec:       imageapi.ImageStreamSpec{DockerImageRepository: from},
		}
		isi = o.newImageStreamImportAll(stream, from)
	} else {
		stream = &imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{Name: o.Name},
			Spec: imageapi.ImageStreamSpec{
				Tags: map[string]imageapi.TagReference{
					tag: {
						From: &kapi.ObjectReference{
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

func (o *ImportImageOptions) getReferencePolicy() imageapi.TagReferencePolicy {
	ref := imageapi.TagReferencePolicy{}
	if len(o.ReferencePolicy) == 0 {
		return ref
	}
	switch o.ReferencePolicy {
	case sourceReferencePolicy:
		ref.Type = imageapi.SourceTagReferencePolicy
	case localReferencePolicy:
		ref.Type = imageapi.LocalTagReferencePolicy
	}
	return ref
}

func (o *ImportImageOptions) newImageStreamImport(stream *imageapi.ImageStream) (*imageapi.ImageStreamImport, bool) {
	isi := &imageapi.ImageStreamImport{
		ObjectMeta: metav1.ObjectMeta{
			Name:            stream.Name,
			Namespace:       o.Namespace,
			ResourceVersion: stream.ResourceVersion,
		},
		Spec: imageapi.ImageStreamImportSpec{Import: !o.DryRun},
	}
	insecureAnnotation := stream.Annotations[imageapi.InsecureRepositoryAnnotation]
	insecure := insecureAnnotation == "true"
	// --insecure flag (if provided) takes precedence over insecure annotation
	if o.Insecure != nil {
		insecure = *o.Insecure
	}

	return isi, insecure
}

func (o *ImportImageOptions) newImageStreamImportAll(stream *imageapi.ImageStream, from string) *imageapi.ImageStreamImport {
	isi, insecure := o.newImageStreamImport(stream)
	isi.Spec.Repository = &imageapi.RepositoryImportSpec{
		From: kapi.ObjectReference{
			Kind: "DockerImage",
			Name: from,
		},
		ImportPolicy: imageapi.TagImportPolicy{
			Insecure:  insecure,
			Scheduled: o.Scheduled,
		},
		ReferencePolicy: o.getReferencePolicy(),
	}

	return isi
}

func (o *ImportImageOptions) newImageStreamImportTags(stream *imageapi.ImageStream, tags map[string]string) *imageapi.ImageStreamImport {
	isi, streamInsecure := o.newImageStreamImport(stream)
	for tag, from := range tags {
		insecure := streamInsecure
		scheduled := o.Scheduled
		oldTag, ok := stream.Spec.Tags[tag]
		if ok {
			insecure = insecure || oldTag.ImportPolicy.Insecure
			scheduled = scheduled || oldTag.ImportPolicy.Scheduled
		}
		isi.Spec.Images = append(isi.Spec.Images, imageapi.ImageImportSpec{
			From: kapi.ObjectReference{
				Kind: "DockerImage",
				Name: from,
			},
			To: &kapi.LocalObjectReference{Name: tag},
			ImportPolicy: imageapi.TagImportPolicy{
				Insecure:  insecure,
				Scheduled: scheduled,
			},
			ReferencePolicy: o.getReferencePolicy(),
		})
	}
	return isi
}

// TransformUnsupportedError converts specific error conditions to unsupported
func TransformUnsupportedError(err error) error {
	if err == nil {
		return nil
	}
	if errors.IsNotFound(err) {
		status, ok := err.(errors.APIStatus)
		if !ok {
			return imageapi.ErrImageStreamImportUnsupported
		}
		if status.Status().Details == nil || status.Status().Details.Kind == "" {
			return imageapi.ErrImageStreamImportUnsupported
		}
	}
	// The ImageStreamImport resource exists in v1.1.1 of origin but is not yet
	// enabled by policy. A create request will return a Forbidden(403) error.
	// We want to return ErrImageStreamImportUnsupported to allow fallback behavior
	// in clients.
	if errors.IsForbidden(err) && !quotautil.IsErrorQuotaExceeded(err) {
		return imageapi.ErrImageStreamImportUnsupported
	}
	return err
}
