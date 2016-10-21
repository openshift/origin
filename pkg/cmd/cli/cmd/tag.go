package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// TagOptions contains all the necessary options for the cli tag command.
type TagOptions struct {
	out      io.Writer
	osClient client.Interface

	deleteTag    bool
	aliasTag     bool
	scheduleTag  bool
	insecureTag  bool
	referenceTag bool
	namespace    string

	ref            imageapi.DockerImageReference
	sourceKind     string
	destNamespace  []string
	destNameAndTag []string
}

var (
	tagLong = templates.LongDesc(`
		Tag existing images into image streams

		The tag command allows you to take an existing tag or image from an image
		stream, or a Docker image pull spec, and set it as the most recent image for a
		tag in 1 or more other image streams. It is similar to the 'docker tag'
		command, but it operates on image streams instead.

		Pass the --insecure flag if your external registry does not have a valid HTTPS
		certificate, or is only served over HTTP. Pass --scheduled to have the server
		regularly check the tag for updates and import the latest version (which can
		then trigger builds and deployments). Note that --scheduled is only allowed for
		Docker images.`)

	tagExample = templates.Examples(`
		# Tag the current image for the image stream 'openshift/ruby' and tag '2.0' into the image stream 'yourproject/ruby with tag 'tip'.
	  %[1]s tag openshift/ruby:2.0 yourproject/ruby:tip

	  # Tag a specific image.
	  %[1]s tag openshift/ruby@sha256:6b646fa6bf5e5e4c7fa41056c27910e679c03ebe7f93e361e6515a9da7e258cc yourproject/ruby:tip

	  # Tag an external Docker image.
	  %[1]s tag --source=docker openshift/origin:latest yourproject/ruby:tip

	  # Remove the specified spec tag from an image stream.
	  %[1]s tag openshift/origin:latest -d`)
)

// NewCmdTag implements the OpenShift cli tag command.
func NewCmdTag(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := &TagOptions{}

	cmd := &cobra.Command{
		Use:     "tag [--source=SOURCETYPE] SOURCE DEST [DEST ...]",
		Short:   "Tag existing images into image streams",
		Long:    tagLong,
		Example: fmt.Sprintf(tagExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, cmd, args, out))
			kcmdutil.CheckErr(opts.Validate())
			kcmdutil.CheckErr(opts.RunTag())
		},
	}

	cmd.Flags().StringVar(&opts.sourceKind, "source", opts.sourceKind, "Optional hint for the source type; valid values are 'imagestreamtag', 'istag', 'imagestreamimage', 'isimage', and 'docker'")
	cmd.Flags().BoolVarP(&opts.deleteTag, "delete", "d", opts.deleteTag, "Delete the provided spec tags")
	cmd.Flags().BoolVar(&opts.aliasTag, "alias", false, "Should the destination tag be updated whenever the source tag changes. Defaults to false.")
	cmd.Flags().BoolVar(&opts.referenceTag, "reference", false, "Should the destination tag continue to pull from the source namespace. Defaults to false.")
	cmd.Flags().BoolVar(&opts.scheduleTag, "scheduled", false, "Set a Docker image to be periodically imported from a remote repository.")
	cmd.Flags().BoolVar(&opts.insecureTag, "insecure", false, "Set to true if importing the specified Docker image requires HTTP or has a self-signed certificate.")

	return cmd
}

func parseStreamName(defaultNamespace, name string) (string, string, error) {
	if !strings.Contains(name, "/") {
		return defaultNamespace, name, nil
	}

	parts := strings.Split(name, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid image stream %q", name)
	}

	namespace := parts[0]
	if len(namespace) == 0 {
		return "", "", fmt.Errorf("invalid namespace %q for image stream %q", namespace, name)
	}

	streamName := parts[1]
	if len(streamName) == 0 {
		return "", "", fmt.Errorf("invalid name %q for image stream %q", streamName, name)
	}

	return namespace, streamName, nil
}

func determineSourceKind(f *clientcmd.Factory, input string) string {
	mapper, _ := f.Object(false)
	gvks, err := mapper.KindsFor(unversioned.GroupVersionResource{Group: imageapi.GroupName, Resource: input})
	if err == nil {
		return gvks[0].Kind
	}

	// DockerImage isn't in RESTMapper
	switch strings.ToLower(input) {
	case "docker", "dockerimage":
		return "DockerImage"
	}

	return input
}

// Complete completes all the required options for the tag command.
func (o *TagOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	if len(args) < 2 && (len(args) < 1 && !o.deleteTag) {
		return kcmdutil.UsageError(cmd, "you must specify a source and at least one destination or one or more tags to delete")
	}

	// Setup writer.
	o.out = out

	// Setup client.
	var err error
	o.osClient, _, err = f.Clients()
	if err != nil {
		return err
	}

	// Setup namespace.
	if len(o.namespace) == 0 {
		o.namespace, _, err = f.DefaultNamespace()
		if err != nil {
			return err
		}
	}

	// Populate source.
	if !o.deleteTag {
		source := args[0]
		glog.V(3).Infof("Using %q as a source tag", source)

		sourceKind := o.sourceKind
		if len(sourceKind) > 0 {
			sourceKind = determineSourceKind(f, sourceKind)
		}
		if len(sourceKind) > 0 {
			validSources := sets.NewString("imagestreamtag", "istag", "imagestreamimage", "isimage", "docker", "dockerimage")
			if !validSources.Has(strings.ToLower(sourceKind)) {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, "invalid source %q; valid values are %v", o.sourceKind, strings.Join(validSources.List(), ", ")))
			}
		}

		ref, err := imageapi.ParseDockerImageReference(source)
		if err != nil {
			return fmt.Errorf("invalid SOURCE: %v", err)
		}
		switch sourceKind {
		case "ImageStreamTag", "ImageStreamImage":
			if len(ref.Registry) > 0 {
				return fmt.Errorf("server in SOURCE is only allowed when providing a Docker image")
			}
			if ref.Namespace == imageapi.DockerDefaultNamespace {
				ref.Namespace = o.namespace
			}
			if sourceKind == "ImageStreamTag" {
				if len(ref.Tag) == 0 {
					return fmt.Errorf("--source=ImageStreamTag requires a valid <name>:<tag> in SOURCE")
				}
			} else {
				if len(ref.ID) == 0 {
					return fmt.Errorf("--source=ImageStreamImage requires a valid <name>@<id> in SOURCE")
				}
			}
		case "":
			if len(ref.Registry) > 0 {
				sourceKind = "DockerImage"
				break
			}
			if len(ref.ID) > 0 {
				sourceKind = "ImageStreamImage"
				break
			}
			if len(ref.Tag) > 0 {
				sourceKind = "ImageStreamTag"
				break
			}
			sourceKind = "DockerImage"
		}

		// if we are not aliasing the tag, specify the exact value to copy
		if sourceKind == "ImageStreamTag" && !o.aliasTag {
			srcNamespace := ref.Namespace
			if len(srcNamespace) == 0 {
				srcNamespace = o.namespace
			}
			is, err := o.osClient.ImageStreams(srcNamespace).Get(ref.Name)
			if err != nil {
				return err
			}
			event := imageapi.LatestTaggedImage(is, ref.Tag)
			if event == nil {
				return fmt.Errorf("%q is not currently pointing to an image, cannot use it as the source of a tag", args[0])
			}
			if len(event.Image) == 0 {
				imageRef, err := imageapi.ParseDockerImageReference(event.DockerImageReference)
				if err != nil {
					return fmt.Errorf("the image stream tag %q has an invalid pull spec and cannot be used to tag: %v", args[0], err)
				}
				ref = imageRef
				sourceKind = "DockerImage"
			} else {
				ref.ID = event.Image
				ref.Tag = ""
				sourceKind = "ImageStreamImage"
			}
		}

		args = args[1:]
		o.sourceKind = sourceKind
		o.ref = ref
		glog.V(3).Infof("Source tag %s %#v", o.sourceKind, o.ref)
	}

	// Populate destinations.
	for _, arg := range args {
		destNamespace, destNameAndTag, err := parseStreamName(o.namespace, arg)
		if err != nil {
			return err
		}
		o.destNamespace = append(o.destNamespace, destNamespace)
		o.destNameAndTag = append(o.destNameAndTag, destNameAndTag)
		glog.V(3).Infof("Using \"%s/%s\" as a destination tag", destNamespace, destNameAndTag)
	}

	return nil
}

// Validate validates all the required options for the tag command.
func (o TagOptions) Validate() error {
	// Validate client and writer
	if o.osClient == nil {
		return errors.New("a client is required")
	}
	if o.out == nil {
		return errors.New("a writer interface is required")
	}

	if o.deleteTag && o.aliasTag {
		return errors.New("--alias and --delete may not be both specified")
	}

	// Validate source tag based on --delete usage.
	if o.deleteTag {
		if len(o.sourceKind) > 0 {
			return errors.New("cannot specify a source kind when deleting")
		}
		if len(o.ref.String()) > 0 {
			return errors.New("cannot specify a source when deleting")
		}
		if o.scheduleTag || o.insecureTag {
			return errors.New("cannot set flags for importing images when deleting a tag")
		}
	} else {
		if len(o.sourceKind) == 0 {
			return errors.New("a source kind is required")
		}
		if len(o.ref.String()) == 0 {
			return errors.New("a source is required")
		}
	}

	// Validate destination tags.
	if len(o.destNamespace) == 0 || len(o.destNameAndTag) == 0 {
		return errors.New("at least a destination is required")
	}
	if len(o.destNamespace) != len(o.destNameAndTag) {
		return errors.New("destination namespaces don't match with destination tags")
	}
	if o.sourceKind != "DockerImage" && (o.scheduleTag || o.insecureTag) {
		return errors.New("only Docker images can have importing flags set")
	}
	if o.aliasTag && (o.scheduleTag || o.insecureTag) {
		return errors.New("cannot set a Docker image tag as an alias and also set import flags")
	}

	return nil
}

// RunTag contains all the necessary functionality for the OpenShift cli tag command.
func (o TagOptions) RunTag() error {
	for i, destNameAndTag := range o.destNameAndTag {
		destName, destTag, ok := imageapi.SplitImageStreamTag(destNameAndTag)
		if !ok {
			return fmt.Errorf("%q must be of the form <stream_name>:<tag>", destNameAndTag)
		}

		err := kclient.RetryOnConflict(kclient.DefaultRetry, func() error {
			isc := o.osClient.ImageStreams(o.destNamespace[i])

			if o.deleteTag {
				// new server support
				err := o.osClient.ImageStreamTags(o.destNamespace[i]).Delete(destName, destTag)
				switch {
				case err == nil:
					fmt.Fprintf(o.out, "Deleted tag %s/%s.", o.destNamespace[i], destNameAndTag)
					return nil

				case kerrors.IsMethodNotSupported(err), kerrors.IsForbidden(err):
					// fall back to legacy behavior
				default:
					//  error that isn't whitelisted: fail
					return err
				}

				// try the old way
				target, err := isc.Get(destName)
				if err != nil {
					if !kerrors.IsNotFound(err) {
						return err
					}

					// Nothing to do here, continue to the next dest tag
					// if there is any.
					fmt.Fprintf(o.out, "Image stream %q does not exist.\n", destName)
					return nil
				}

				// The user wants to delete a spec tag.
				if _, ok := target.Spec.Tags[destTag]; !ok {
					return fmt.Errorf("destination tag %s/%s does not exist.\n", o.destNamespace[i], destNameAndTag)
				}
				delete(target.Spec.Tags, destTag)

				if _, err = isc.Update(target); err != nil {
					return err
				}

				fmt.Fprintf(o.out, "Deleted tag %s/%s.", o.destNamespace[i], destNameAndTag)
				return nil
			}

			// The user wants to symlink a tag.
			istag := &imageapi.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{
					Name:      destNameAndTag,
					Namespace: o.destNamespace[i],
				},
				Tag: &imageapi.TagReference{
					Reference: o.referenceTag,
					ImportPolicy: imageapi.TagImportPolicy{
						Insecure:  o.insecureTag,
						Scheduled: o.scheduleTag,
					},
					From: &kapi.ObjectReference{
						Kind: o.sourceKind,
					},
				},
			}
			localRef := o.ref
			switch o.sourceKind {
			case "DockerImage":
				istag.Tag.From.Name = localRef.Exact()
				gen := int64(0)
				istag.Tag.Generation = &gen

			default:
				istag.Tag.From.Name = localRef.NameString()
				istag.Tag.From.Namespace = o.ref.Namespace
				if len(o.ref.Namespace) == 0 && o.destNamespace[i] != o.namespace {
					istag.Tag.From.Namespace = o.namespace
				}
			}

			msg := ""
			sameNamespace := o.namespace == o.destNamespace[i]
			if o.aliasTag {
				if sameNamespace {
					msg = fmt.Sprintf("Tag %s set up to track %s.", destNameAndTag, o.ref.Exact())
				} else {
					msg = fmt.Sprintf("Tag %s/%s set up to track %s.", o.destNamespace[i], destNameAndTag, o.ref.Exact())
				}
			} else {
				if istag.Tag.ImportPolicy.Scheduled {
					if sameNamespace {
						msg = fmt.Sprintf("Tag %s set to import %s periodically.", destNameAndTag, o.ref.Exact())
					} else {
						msg = fmt.Sprintf("Tag %s/%s set to %s periodically.", o.destNamespace[i], destNameAndTag, o.ref.Exact())
					}
				} else {
					if sameNamespace {
						msg = fmt.Sprintf("Tag %s set to %s.", destNameAndTag, o.ref.Exact())
					} else {
						msg = fmt.Sprintf("Tag %s/%s set to %s.", o.destNamespace[i], destNameAndTag, o.ref.Exact())
					}
				}
			}

			// supported by new servers.
			_, err := o.osClient.ImageStreamTags(o.destNamespace[i]).Update(istag)
			switch {
			case err == nil:
				fmt.Fprintln(o.out, msg)
				return nil

			case kerrors.IsMethodNotSupported(err), kerrors.IsForbidden(err), kerrors.IsNotFound(err):
				// if we got one of these errors, it possible that a Create will do what we need.  Try that
				_, err := o.osClient.ImageStreamTags(o.destNamespace[i]).Create(istag)
				switch {
				case err == nil:
					fmt.Fprintln(o.out, msg)
					return nil

				case kerrors.IsMethodNotSupported(err), kerrors.IsForbidden(err):
					// fall back to legacy behavior
				default:
					//  error that isn't whitelisted: fail
					return err
				}

			default:
				//  error that isn't whitelisted: fail
				return err

			}

			target, err := isc.Get(destName)
			if kerrors.IsNotFound(err) {
				target = &imageapi.ImageStream{
					ObjectMeta: kapi.ObjectMeta{
						Name: destName,
					},
				}
			} else if err != nil {
				return err
			}

			if target.Spec.Tags == nil {
				target.Spec.Tags = make(map[string]imageapi.TagReference)
			}

			if oldTargetTag, exists := target.Spec.Tags[destTag]; exists {
				if oldTargetTag.Generation == nil {
					// for servers that do not support tag generations, we need to force re-import to fetch its metadata
					delete(target.Annotations, imageapi.DockerImageRepositoryCheckAnnotation)
					istag.Tag.Generation = nil
				}
			}
			target.Spec.Tags[destTag] = *istag.Tag

			// Check the stream creation timestamp and make sure we will not
			// create a new image stream while deleting.
			if target.CreationTimestamp.IsZero() && !o.deleteTag {
				_, err = isc.Create(target)
			} else {
				_, err = isc.Update(target)
			}
			if err != nil {
				return err
			}

			fmt.Fprintln(o.out, msg)
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}
