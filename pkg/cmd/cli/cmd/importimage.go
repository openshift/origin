package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	kctl "k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/templates"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

var (
	importImageLong = templates.LongDesc(`
		Import tag and image information from an external Docker image repository

		Only image streams that have a value set for spec.dockerImageRepository and/or
		spec.Tags may have tag and image information imported.`)

	importImageExample = templates.Examples(`  %[1]s import-image mystream`)
)

// NewCmdImportImage implements the OpenShift cli import-image command.
func NewCmdImportImage(fullName string, f *clientcmd.Factory, out, errout io.Writer) *cobra.Command {
	opts := &ImportImageOptions{}
	cmd := &cobra.Command{
		Use:        "import-image IMAGESTREAM[:TAG]",
		Short:      "Imports images from a Docker registry",
		Long:       importImageLong,
		Example:    fmt.Sprintf(importImageExample, fullName),
		SuggestFor: []string{"image"},
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, cmd, args, fullName, out, errout))
			kcmdutil.CheckErr(opts.Validate(cmd))
			kcmdutil.CheckErr(opts.Run())
		},
	}
	cmd.Flags().StringVar(&opts.From, "from", "", "A Docker image repository to import images from")
	cmd.Flags().BoolVar(&opts.Confirm, "confirm", false, "If true, allow the image stream import location to be set or changed")
	cmd.Flags().BoolVar(&opts.All, "all", false, "If true, import all tags from the provided source on creation or if --from is specified")
	opts.Insecure = cmd.Flags().Bool("insecure", false, "If true, allow importing from registries that have invalid HTTPS certificates or are hosted via HTTP. This flag will take precedence over the insecure annotation.")

	return cmd
}

// ImageImportOptions contains all the necessary information to perform an import.
type ImportImageOptions struct {
	// user set values
	From     string
	Confirm  bool
	All      bool
	Insecure *bool

	// internal values
	Namespace string
	Name      string
	Tag       string
	Target    string

	CommandName string

	// helpers
	out      io.Writer
	errout   io.Writer
	osClient client.Interface
	isClient client.ImageStreamInterface
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

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace

	osClient, _, err := f.Clients()
	if err != nil {
		return err
	}
	o.osClient = osClient
	o.isClient = osClient.ImageStreams(namespace)
	o.out = out
	o.errout = errout

	return nil
}

// Validate ensures that a ImportImageOptions is valid and can be used to execute
// an import.
func (o *ImportImageOptions) Validate(cmd *cobra.Command) error {
	if len(o.Target) == 0 {
		return kcmdutil.UsageError(cmd, "you must specify the name of an image stream")
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
	// TODO: add dry-run
	stream, isi, err := o.createImageImport()
	if err != nil {
		return err
	}

	// Attempt the new, direct import path
	result, err := o.isClient.Import(isi)
	switch {
	case err == client.ErrImageStreamImportUnsupported:
	case err != nil:
		return err
	default:
		if wasError(result) {
			fmt.Fprintf(o.errout, "The import completed with errors.\n\n")
		} else {
			fmt.Fprint(o.out, "The import completed successfully.\n\n")
		}

		// optimization, use the image stream returned by the call
		d := describe.ImageStreamDescriber{Interface: o.osClient}
		info, err := d.Describe(o.Namespace, stream.Name, kctl.DescriberSettings{})
		if err != nil {
			return err
		}

		fmt.Fprintln(o.out, info)

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

	d := describe.ImageStreamDescriber{Interface: o.osClient}
	info, err := d.Describe(updatedStream.Namespace, updatedStream.Name, kctl.DescriberSettings{})
	if err != nil {
		return err
	}

	fmt.Fprintln(o.out, info)
	return nil
}

func wasError(isi *imageapi.ImageStreamImport) bool {
	for _, image := range isi.Status.Images {
		if image.Status.Status == unversioned.StatusFailure {
			return true
		}
	}
	if isi.Status.Repository != nil && isi.Status.Repository.Status.Status == unversioned.StatusFailure {
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
	streamWatch, err := o.isClient.Watch(kapi.ListOptions{FieldSelector: fields.OneTermEqualSelector("metadata.name", o.Name), ResourceVersion: resourceVersion})
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
	stream, err := o.isClient.Get(o.Name)
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

	// the stream already exists
	if len(stream.Spec.DockerImageRepository) == 0 && len(stream.Spec.Tags) == 0 {
		return nil, nil, fmt.Errorf("image stream does not have valid docker images to be imported")
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
	finalTag, existing, ok, multiple := imageapi.FollowTagReference(stream, tag)
	if !ok && multiple {
		return nil, fmt.Errorf("tag %q on the image stream is a reference to %q, which does not exist", tag, finalTag)
	}

	// update ImageStream appropriately
	if ok {
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
				return nil, fmt.Errorf("the tag %q points to the tag %q which points to %q - use the 'tag' command if you want to change the source to %q", tag, finalTag, existing.From.Name, from)
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

	} else {
		// create a new tag
		if len(from) == 0 {
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
	var stream *imageapi.ImageStream
	// create new ImageStream
	if o.All {
		stream = &imageapi.ImageStream{
			ObjectMeta: kapi.ObjectMeta{Name: o.Name},
			Spec:       imageapi.ImageStreamSpec{DockerImageRepository: from},
		}
	} else {
		stream = &imageapi.ImageStream{
			ObjectMeta: kapi.ObjectMeta{Name: o.Name},
			Spec: imageapi.ImageStreamSpec{
				Tags: map[string]imageapi.TagReference{
					tag: {
						From: &kapi.ObjectReference{
							Kind: "DockerImage",
							Name: from,
						},
					},
				},
			},
		}
	}
	// and accompanying ImageStreamImport
	var isi *imageapi.ImageStreamImport
	if o.All {
		isi = o.newImageStreamImportAll(stream, from)
	} else {
		isi = o.newImageStreamImportTags(stream, map[string]string{tag: from})
	}

	return stream, isi
}

func (o *ImportImageOptions) newImageStreamImport(stream *imageapi.ImageStream) (*imageapi.ImageStreamImport, bool) {
	isi := &imageapi.ImageStreamImport{
		ObjectMeta: kapi.ObjectMeta{
			Name:            stream.Name,
			Namespace:       o.Namespace,
			ResourceVersion: stream.ResourceVersion,
		},
		Spec: imageapi.ImageStreamImportSpec{Import: true},
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
		ImportPolicy: imageapi.TagImportPolicy{Insecure: insecure},
	}

	return isi
}

func (o *ImportImageOptions) newImageStreamImportTags(stream *imageapi.ImageStream, tags map[string]string) *imageapi.ImageStreamImport {
	isi, insecure := o.newImageStreamImport(stream)
	for tag, from := range tags {
		isi.Spec.Images = append(isi.Spec.Images, imageapi.ImageImportSpec{
			From: kapi.ObjectReference{
				Kind: "DockerImage",
				Name: from,
			},
			To:           &kapi.LocalObjectReference{Name: tag},
			ImportPolicy: imageapi.TagImportPolicy{Insecure: insecure},
		})
	}
	return isi
}
