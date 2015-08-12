package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const (
	tagLong = `
Tag existing images into image streams

The tag command allows you to take an existing tag or image from an image
stream, or a Docker image pull spec, and set it as the most recent image for a
tag in 1 or more other image streams. It is similar to the 'docker tag'
command, but it operates on image streams instead.`

	tagExample = `  // Tag the current image for the image stream 'openshift/ruby' and tag '2.0' into the image stream 'yourproject/ruby with tag 'tip':
  $ %[1]s tag openshift/ruby:2.0 yourproject/ruby:tip

  // Tag a specific image:
  $ %[1]s tag openshift/ruby@sha256:6b646fa6bf5e5e4c7fa41056c27910e679c03ebe7f93e361e6515a9da7e258cc yourproject/ruby:tip

  // Tag an external Docker image:
  $ %[1]s tag --source=docker openshift/origin:latest yourproject/ruby:tip`
)

// NewCmdTag implements the OpenShift cli tag command.
func NewCmdTag(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	var sourceKind string

	cmd := &cobra.Command{
		Use:     "tag [--source=SOURCETYPE] SOURCE DEST [DEST ...]",
		Short:   "Tag existing images into image streams",
		Long:    tagLong,
		Example: fmt.Sprintf(tagExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunTag(f, out, cmd, args, sourceKind)
			cmdutil.CheckErr(err)
		},
	}

	cmd.Flags().StringVar(&sourceKind, "source", sourceKind, "Optional hint for the source type; valid values are 'imagestreamtag', 'istag', 'imagestreamimage', 'isimage', and 'docker'")

	return cmd
}

func parseStreamName(name, defaultNamespace string) (string, string, error) {
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

// RunTag contains all the necessary functionality for the OpenShift cli tag command.
func RunTag(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string, sourceKind string) error {
	if len(args) < 2 {
		return cmdutil.UsageError(cmd, "you must specify a source and at least one destination")
	}

	original := sourceKind
	if len(sourceKind) > 0 {
		sourceKind = determineSourceKind(f, sourceKind)
	}
	if len(sourceKind) > 0 {
		validSources := util.NewStringSet("imagestreamtag", "istag", "imagestreamimage", "isimage", "docker", "dockerimage")
		if !validSources.Has(strings.ToLower(sourceKind)) {
			cmdutil.CheckErr(cmdutil.UsageError(cmd, "invalid source %q; valid values are %v", original, strings.Join(validSources.List(), ", ")))
		}
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	ref, err := imageapi.ParseDockerImageReference(args[0])
	if err != nil {
		return fmt.Errorf("invalid SOURCE: %v", err)
	}
	switch sourceKind {
	case "ImageStreamTag", "ImageStreamImage":
		if len(ref.Registry) > 0 {
			return fmt.Errorf("server in SOURCE is only allowed when providing a Docker image")
		}
		if ref.Namespace == imageapi.DockerDefaultNamespace {
			ref.Namespace = namespace
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

	osClient, _, err := f.Clients()
	if err != nil {
		return err
	}

	glog.V(4).Infof("Source tag %s %#v", sourceKind, ref)

	for _, arg := range args[1:] {
		destNamespace, destNameAndTag, err := parseStreamName(arg, namespace)
		if err != nil {
			return err
		}

		destName, destTag, ok := imageapi.SplitImageStreamTag(destNameAndTag)
		if !ok {
			return fmt.Errorf("%q must be of the form <namespace>/<stream_name>:<tag>", arg)
		}

		isc := osClient.ImageStreams(destNamespace)

		target, err := isc.Get(destName)
		if err != nil {
			if !kerrors.IsNotFound(err) {
				return err
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

		targetRef, ok := target.Spec.Tags[destTag]
		if !ok {
			targetRef = imageapi.TagReference{}
		}

		targetRef.From = &kapi.ObjectReference{
			Kind: sourceKind,
		}
		localRef := ref
		switch sourceKind {
		case "DockerImage":
			targetRef.From.Name = localRef.String()
		default:
			targetRef.From.Name = localRef.NameString()
			targetRef.From.Namespace = ref.Namespace
		}

		target.Spec.Tags[destTag] = targetRef

		if target.CreationTimestamp.IsZero() {
			_, err = isc.Create(target)
		} else {
			_, err = isc.Update(target)
		}
		if err != nil {
			return err
		}
	}

	return nil
}
