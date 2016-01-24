package cmd

import (
	"fmt"
	"io"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	importImageLong = `
Import tag and image information from an external Docker image repository

Only image streams that have a value set for spec.dockerImageRepository and/or
spec.Tags may have tag and image information imported.`

	importImageExample = `  $ %[1]s import-image mystream`
)

// NewCmdImportImage implements the OpenShift cli import-image command.
func NewCmdImportImage(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "import-image IMAGESTREAM[:TAG]",
		Short:      "Imports images from a Docker registry",
		Long:       importImageLong,
		Example:    fmt.Sprintf(importImageExample, fullName),
		SuggestFor: []string{"image"},
		Run: func(cmd *cobra.Command, args []string) {
			err := RunImportImage(f, out, cmd, args)
			cmdutil.CheckErr(err)
		},
	}
	cmd.Flags().String("from", "", "A Docker image repository or tag to import images from")
	cmd.Flags().Bool("confirm", false, "If true, allow the image stream import location to be set or changed")
	cmd.Flags().Bool("all", false, "If true, import all tags from the provided source on creation or if --from is specified")
	cmd.Flags().Bool("insecure", false, "If true, allow importing from registries that have invalid HTTPS certificates or are hosted via HTTP")

	return cmd
}

// RunImportImage contains all the necessary functionality for the OpenShift cli import-image command.
func RunImportImage(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	if len(args) == 0 || len(args[0]) == 0 {
		return cmdutil.UsageError(cmd, "you must specify the name of an image stream")
	}

	target := args[0]
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	osClient, _, err := f.Clients()
	if err != nil {
		return err
	}

	insecure := cmdutil.GetFlagBool(cmd, "insecure")
	from := cmdutil.GetFlagString(cmd, "from")
	confirm := cmdutil.GetFlagBool(cmd, "confirm")
	all := cmdutil.GetFlagBool(cmd, "all")

	targetRef, err := imageapi.ParseDockerImageReference(target)
	switch {
	case err != nil:
		return fmt.Errorf("the image name must be a valid Docker image pull spec or reference to an image stream (e.g. myregistry/myteam/image:tag)")
	case len(targetRef.ID) > 0:
		return fmt.Errorf("to import images by ID, use the 'tag' command")
	case len(targetRef.Tag) != 0 && all:
		// error out
		return fmt.Errorf("cannot specify a tag %q as well as --all", target)
	case len(targetRef.Tag) == 0 && !all:
		// apply the default tag
		targetRef.Tag = imageapi.DefaultImageTag
	}
	name := targetRef.Name
	tag := targetRef.Tag

	imageStreamClient := osClient.ImageStreams(namespace)
	stream, err := imageStreamClient.Get(name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		// the stream is new
		if !confirm {
			return fmt.Errorf("no image stream named %q exists, pass --confirm to create and import", name)
		}
		if len(from) == 0 {
			from = target
		}
		if all {
			stream = &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: name},
				Spec:       imageapi.ImageStreamSpec{DockerImageRepository: from},
			}
		} else {
			stream = &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: name},
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

	} else {
		// the stream already exists
		if len(stream.Spec.DockerImageRepository) == 0 && len(stream.Spec.Tags) == 0 {
			return fmt.Errorf("image stream has not defined anything to import")
		}

		if all {
			// importing a whole repository
			if len(from) == 0 {
				from = target
			}
			if from != stream.Spec.DockerImageRepository {
				if !confirm {
					if len(stream.Spec.DockerImageRepository) == 0 {
						return fmt.Errorf("the image stream does not currently import an entire Docker repository, pass --confirm to update")
					}
					return fmt.Errorf("the image stream has a different import spec %q, pass --confirm to update", stream.Spec.DockerImageRepository)
				}
				stream.Spec.DockerImageRepository = from
			}

		} else {
			// importing a single tag

			// follow any referential tags to the destination
			finalTag, existing, ok, multiple := imageapi.FollowTagReference(stream, tag)
			if !ok && multiple {
				return fmt.Errorf("tag %q on the image stream is a reference to %q, which does not exist", tag, finalTag)
			}

			if ok {
				// disallow changing an existing tag
				if existing.From == nil || existing.From.Kind != "DockerImage" {
					return fmt.Errorf("tag %q already exists - you must use the 'tag' command if you want to change the source to %q", tag, from)
				}
				if len(from) != 0 && from != existing.From.Name {
					if multiple {
						return fmt.Errorf("the tag %q points to the tag %q which points to %q - use the 'tag' command if you want to change the source to %q", tag, finalTag, existing.From.Name, from)
					}
					return fmt.Errorf("the tag %q points to %q - use the 'tag' command if you want to change the source to %q", tag, existing.From.Name, from)
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
					from = target
				}
				existing = &imageapi.TagReference{
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: from,
					},
				}
			}
			stream.Spec.Tags[tag] = *existing
		}
	}

	if len(from) == 0 {
		// catch programmer error
		return fmt.Errorf("unexpected error, from is empty")
	}

	// Attempt the new, direct import path
	isi := &imageapi.ImageStreamImport{
		ObjectMeta: kapi.ObjectMeta{
			Name:            stream.Name,
			Namespace:       namespace,
			ResourceVersion: stream.ResourceVersion,
		},
		Spec: imageapi.ImageStreamImportSpec{Import: true},
	}
	if all {
		isi.Spec.Repository = &imageapi.RepositoryImportSpec{
			From: kapi.ObjectReference{
				Kind: "DockerImage",
				Name: from,
			},
			ImportPolicy: imageapi.TagImportPolicy{Insecure: insecure},
		}
	} else {
		isi.Spec.Images = append(isi.Spec.Images, imageapi.ImageImportSpec{
			From: kapi.ObjectReference{
				Kind: "DockerImage",
				Name: from,
			},
			To:           &kapi.LocalObjectReference{Name: tag},
			ImportPolicy: imageapi.TagImportPolicy{Insecure: insecure},
		})
	}

	// TODO: add dry-run
	_, err = imageStreamClient.Import(isi)
	switch {
	case err == client.ErrImageStreamImportUnsupported:
	case err != nil:
		return err
	default:
		fmt.Fprint(cmd.Out(), "The import completed successfully.\n\n")

		// optimization, use the image stream returned by the call
		d := describe.ImageStreamDescriber{Interface: osClient}
		info, err := d.Describe(namespace, stream.Name)
		if err != nil {
			return err
		}

		fmt.Fprintln(out, info)
		return nil
	}

	// Legacy path, remove when support for older importers is removed
	delete(stream.Annotations, imageapi.DockerImageRepositoryCheckAnnotation)
	if insecure {
		if stream.Annotations == nil {
			stream.Annotations = make(map[string]string)
		}
		stream.Annotations[imageapi.InsecureRepositoryAnnotation] = "true"
	}

	if stream.CreationTimestamp.IsZero() {
		stream, err = imageStreamClient.Create(stream)
	} else {
		stream, err = imageStreamClient.Update(stream)
	}
	if err != nil {
		return err
	}

	resourceVersion := stream.ResourceVersion

	fmt.Fprintln(cmd.Out(), "Importing (ctrl+c to stop waiting) ...")

	updatedStream, err := waitForImport(imageStreamClient, stream.Name, resourceVersion)
	if err != nil {
		if _, ok := err.(importError); ok {
			return err
		}
		return fmt.Errorf("unable to determine if the import completed successfully - please run 'oc describe -n %s imagestream/%s' to see if the tags were updated as expected: %v", stream.Namespace, stream.Name, err)
	}

	fmt.Fprint(cmd.Out(), "The import completed successfully.\n\n")

	d := describe.ImageStreamDescriber{Interface: osClient}
	info, err := d.Describe(updatedStream.Namespace, updatedStream.Name)
	if err != nil {
		return err
	}

	fmt.Fprintln(out, info)
	return nil
}

// TODO: move to image/api as a helper
type importError struct {
	annotation string
}

func (e importError) Error() string {
	return fmt.Sprintf("unable to import image: %s", e.annotation)
}

func waitForImport(imageStreamClient client.ImageStreamInterface, name, resourceVersion string) (*imageapi.ImageStream, error) {
	streamWatch, err := imageStreamClient.Watch(labels.Everything(), fields.OneTermEqualSelector("metadata.name", name), resourceVersion)
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
