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
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// TagOptions contains all the necessary options for the cli tag command.
type TagOptions struct {
	out      io.Writer
	osClient client.Interface

	deleteTag bool
	aliasTag  bool

	ref            imageapi.DockerImageReference
	sourceKind     string
	destNamespace  []string
	destNameAndTag []string
}

const (
	tagLong = `
Tag existing images into image streams

The tag command allows you to take an existing tag or image from an image
stream, or a Docker image pull spec, and set it as the most recent image for a
tag in 1 or more other image streams. It is similar to the 'docker tag'
command, but it operates on image streams instead.`

	tagExample = `  # Tag the current image for the image stream 'openshift/ruby' and tag '2.0' into the image stream 'yourproject/ruby with tag 'tip'.
  $ %[1]s tag openshift/ruby:2.0 yourproject/ruby:tip

  # Tag a specific image.
  $ %[1]s tag openshift/ruby@sha256:6b646fa6bf5e5e4c7fa41056c27910e679c03ebe7f93e361e6515a9da7e258cc yourproject/ruby:tip

  # Tag an external Docker image.
  $ %[1]s tag --source=docker openshift/origin:latest yourproject/ruby:tip

  # Remove the specified spec tag from an image stream.
  $ %[1]s tag openshift/origin:latest -d`
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
			cmdutil.CheckErr(opts.Complete(f, cmd, args, out))
			cmdutil.CheckErr(opts.Validate())
			cmdutil.CheckErr(opts.RunTag())
		},
	}

	cmd.Flags().StringVar(&opts.sourceKind, "source", opts.sourceKind, "Optional hint for the source type; valid values are 'imagestreamtag', 'istag', 'imagestreamimage', 'isimage', and 'docker'")
	cmd.Flags().BoolVarP(&opts.deleteTag, "delete", "d", opts.deleteTag, "Delete the provided spec tags")
	cmd.Flags().BoolVar(&opts.aliasTag, "alias", false, "Should the destination tag be updated whenever the source tag changes. Defaults to false.")

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
	mapper, _ := f.Object()
	_, kind, err := mapper.VersionAndKindForResource(input)
	if err == nil {
		return kind
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
		return cmdutil.UsageError(cmd, "you must specify a source and at least one destination or one or more tags to delete")
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
	defaultNamespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
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
				cmdutil.CheckErr(cmdutil.UsageError(cmd, "invalid source %q; valid values are %v", o.sourceKind, strings.Join(validSources.List(), ", ")))
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
				ref.Namespace = defaultNamespace
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
				srcNamespace = defaultNamespace
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
		destNamespace, destNameAndTag, err := parseStreamName(defaultNamespace, arg)
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
		return errors.New("--alias and --delete may not both be specified")
	}

	// Validate source tag based on --delete usage.
	if o.deleteTag {
		if len(o.sourceKind) > 0 {
			return errors.New("cannot specify a source kind when deleting")
		}
		if len(o.ref.String()) > 0 {
			return errors.New("cannot specify a source when deleting")
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

	return nil
}

// RunTag contains all the necessary functionality for the OpenShift cli tag command.
func (o TagOptions) RunTag() error {
	for i, destNameAndTag := range o.destNameAndTag {
		destName, destTag, ok := imageapi.SplitImageStreamTag(destNameAndTag)
		if !ok {
			return fmt.Errorf("%q must be of the form <namespace>/<stream_name>:<tag>", destNameAndTag)
		}

		isc := o.osClient.ImageStreams(o.destNamespace[i])
		target, err := isc.Get(destName)
		if err != nil {
			if !kerrors.IsNotFound(err) {
				return err
			}

			if o.deleteTag {
				// Nothing to do here, continue to the next dest tag
				// if there is any.
				continue
			}

			// try to create the target if it doesn't exist
			target = &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name: destName,
				},
			}
		}

		if target.Spec.Tags == nil {
			target.Spec.Tags = make(map[string]imageapi.TagReference)
		}

		msg := ""
		if !o.deleteTag {
			// The user wants to symlink a tag.
			targetRef, ok := target.Spec.Tags[destTag]
			if !ok {
				targetRef = imageapi.TagReference{}
			}

			targetRef.From = &kapi.ObjectReference{
				Kind: o.sourceKind,
			}
			localRef := o.ref
			switch o.sourceKind {
			case "DockerImage":
				targetRef.From.Name = localRef.String()
			default:
				targetRef.From.Name = localRef.NameString()
				targetRef.From.Namespace = o.ref.Namespace
			}

			target.Spec.Tags[destTag] = targetRef
			msg = fmt.Sprintf("Tag %s set up to track tag %s/%s.", o.ref, o.destNamespace[i], destNameAndTag)
		} else {
			// The user wants to delete a spec tag.
			if _, ok := target.Spec.Tags[destTag]; !ok {
				glog.V(4).Infof("Destination tag %s/%s does not exist", o.destNamespace[i], destNameAndTag)
				return nil
			}
			delete(target.Spec.Tags, destTag)
			msg = fmt.Sprintf("Deleted tag %s/%s.", o.destNamespace[i], destNameAndTag)
		}

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
	}

	return nil
}
