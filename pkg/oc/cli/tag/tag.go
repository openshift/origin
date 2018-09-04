package tag

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	imagev1 "github.com/openshift/api/image/v1"
	imagev1typedclient "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageutil "github.com/openshift/origin/pkg/image/util"
)

// TagOptions contains all the necessary options for the cli tag command.
type TagOptions struct {
	client imagev1typedclient.ImageV1Interface

	deleteTag    bool
	aliasTag     bool
	scheduleTag  bool
	insecureTag  bool
	referenceTag bool
	namespace    string

	referencePolicy string

	ref            imagev1.DockerImageReference
	sourceKind     string
	destNamespace  []string
	destNameAndTag []string

	genericclioptions.IOStreams
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
	  %[1]s tag --source=docker openshift/origin-control-plane:latest yourproject/ruby:tip

	  # Tag an external Docker image and request pullthrough for it.
	  %[1]s tag --source=docker openshift/origin-control-plane:latest yourproject/ruby:tip --reference-policy=local

	  # Remove the specified spec tag from an image stream.
	  %[1]s tag openshift/origin-control-plane:latest -d`)
)

const (
	SourceReferencePolicy = "source"
	LocalReferencePolicy  = "local"
)

func NewTagOptions(streams genericclioptions.IOStreams) *TagOptions {
	return &TagOptions{
		IOStreams: streams,
	}
}

// NewCmdTag implements the OpenShift cli tag command.
func NewCmdTag(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewTagOptions(streams)
	cmd := &cobra.Command{
		Use:     "tag [--source=SOURCETYPE] SOURCE DEST [DEST ...]",
		Short:   "Tag existing images into image streams",
		Long:    tagLong,
		Example: fmt.Sprintf(tagExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.sourceKind, "source", o.sourceKind, "Optional hint for the source type; valid values are 'imagestreamtag', 'istag', 'imagestreamimage', 'isimage', and 'docker'.")
	cmd.Flags().BoolVarP(&o.deleteTag, "delete", "d", o.deleteTag, "Delete the provided spec tags.")
	cmd.Flags().BoolVar(&o.aliasTag, "alias", o.aliasTag, "Should the destination tag be updated whenever the source tag changes. Applies only to a single image stream. Defaults to false.")
	cmd.Flags().BoolVar(&o.referenceTag, "reference", o.referenceTag, "Should the destination tag continue to pull from the source namespace. Defaults to false.")
	cmd.Flags().BoolVar(&o.scheduleTag, "scheduled", o.scheduleTag, "Set a Docker image to be periodically imported from a remote repository. Defaults to false.")
	cmd.Flags().BoolVar(&o.insecureTag, "insecure", o.insecureTag, "Set to true if importing the specified Docker image requires HTTP or has a self-signed certificate. Defaults to false.")
	cmd.Flags().StringVar(&o.referencePolicy, "reference-policy", SourceReferencePolicy, "Allow to request pullthrough for external image when set to 'local'. Defaults to 'source'.")

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

func determineSourceKind(f kcmdutil.Factory, input string) (string, error) {
	mapper, err := f.ToRESTMapper()
	if err != nil {
		return "", err
	}
	gvks, err := mapper.KindsFor(schema.GroupVersionResource{Group: imagev1.GroupName, Resource: input})
	if err == nil {
		return gvks[0].Kind, nil
	}

	// DockerImage isn't in RESTMapper
	switch strings.ToLower(input) {
	case "docker", "dockerimage":
		return "DockerImage", nil
	}

	return input, nil
}

// Complete completes all the required options for the tag command.
func (o *TagOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) < 2 && (len(args) < 1 && !o.deleteTag) {
		return kcmdutil.UsageErrorf(cmd, "you must specify a source and at least one destination or one or more tags to delete")
	}

	// Setup clients.
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.client, err = imagev1typedclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	// Populate source.
	if !o.deleteTag {
		source := args[0]
		glog.V(3).Infof("Using %q as a source tag", source)

		sourceKind := o.sourceKind
		if len(sourceKind) > 0 {
			sourceKind, err = determineSourceKind(f, sourceKind)
			if err != nil {
				return err
			}
		}
		if len(sourceKind) > 0 {
			validSources := sets.NewString("imagestreamtag", "istag", "imagestreamimage", "isimage", "docker", "dockerimage")
			if !validSources.Has(strings.ToLower(sourceKind)) {
				return kcmdutil.UsageErrorf(cmd, "invalid source %q; valid values are %v", o.sourceKind, strings.Join(validSources.List(), ", "))
			}
		}

		ref, err := imageutil.ParseDockerImageReference(source)
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
			is, err := o.client.ImageStreams(srcNamespace).Get(ref.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			event := imageutil.LatestTaggedImage(is, ref.Tag)
			if event == nil {
				return fmt.Errorf("%q is not currently pointing to an image, cannot use it as the source of a tag", args[0])
			}
			if len(event.Image) == 0 {
				imageRef, err := imageutil.ParseDockerImageReference(event.DockerImageReference)
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

// isCrossImageStream verifies if destination is the same image stream as source. Returns true
// if any of the destination image stream is different and error from parsing
// image stream tag.
func isCrossImageStream(namespace string, srcRef imagev1.DockerImageReference, destNamespace []string, destNameAndTag []string) (bool, error) {
	for i, ns := range destNamespace {
		if namespace != ns {
			return true, nil
		}
		name, _, ok := imageapi.SplitImageStreamTag(destNameAndTag[i])
		if !ok {
			return false, fmt.Errorf("%q must be of the form <stream_name>:<tag>", destNameAndTag[i])
		}
		if srcRef.Name != name {
			return true, nil
		}
	}
	return false, nil
}

// Validate validates all the required options for the tag command.
func (o TagOptions) Validate() error {
	if o.deleteTag && o.aliasTag {
		return errors.New("--alias and --delete may not be both specified")
	}

	if o.referencePolicy != SourceReferencePolicy && o.referencePolicy != LocalReferencePolicy {
		return errors.New("reference policy must be set to 'source' or 'local'")
	}

	// Validate source tag based on --delete usage.
	if o.deleteTag {
		if len(o.sourceKind) > 0 {
			return errors.New("cannot specify a source kind when deleting")
		}
		if len(imageutil.DockerImageReferenceString(o.ref)) > 0 {
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
	if o.aliasTag {
		if o.scheduleTag || o.insecureTag {
			return errors.New("cannot set a Docker image tag as an alias and also set import flags")
		}
		cross, err := isCrossImageStream(o.namespace, o.ref, o.destNamespace, o.destNameAndTag)
		if err != nil {
			return err
		}
		if cross {
			return errors.New("cannot set alias across different Image Streams")
		}
	}

	return nil
}

// Run contains all the necessary functionality for the OpenShift cli tag command.
func (o TagOptions) Run() error {
	var tagReferencePolicy imagev1.TagReferencePolicyType
	switch o.referencePolicy {
	case SourceReferencePolicy:
		tagReferencePolicy = imagev1.SourceTagReferencePolicy
	case LocalReferencePolicy:
		tagReferencePolicy = imagev1.LocalTagReferencePolicy
	}
	for i, destNameAndTag := range o.destNameAndTag {
		destName, destTag, ok := imageapi.SplitImageStreamTag(destNameAndTag)
		if !ok {
			return fmt.Errorf("%q must be of the form <stream_name>:<tag>", destNameAndTag)
		}

		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			isc := o.client.ImageStreams(o.destNamespace[i])

			if o.deleteTag {
				// new server support
				err := o.client.ImageStreamTags(o.destNamespace[i]).Delete(imageapi.JoinImageStreamTag(destName, destTag), &metav1.DeleteOptions{})
				switch {
				case err == nil:
					fmt.Fprintf(o.Out, "Deleted tag %s/%s.\n", o.destNamespace[i], destNameAndTag)
					return nil

				case kerrors.IsMethodNotSupported(err), kerrors.IsForbidden(err):
					// fall back to legacy behavior
				default:
					//  error that isn't whitelisted: fail
					return err
				}

				// try the old way
				target, err := isc.Get(destName, metav1.GetOptions{})
				if err != nil {
					if !kerrors.IsNotFound(err) {
						return err
					}

					// Nothing to do here, continue to the next dest tag
					// if there is any.
					fmt.Fprintf(o.Out, "Image stream %q does not exist.\n", destName)
					return nil
				}

				// The user wants to delete a spec tag.
				if _, ok := imageutil.SpecHasTag(target, destTag); !ok {
					return fmt.Errorf("destination tag %s/%s does not exist.\n", o.destNamespace[i], destNameAndTag)
				}
				// delete tag
				tags := []imagev1.TagReference{}
				for i := range target.Spec.Tags {
					t := target.Spec.Tags[i]
					if t.Name != destTag {
						tags = append(tags, t)
					}
				}
				target.Spec.Tags = tags

				if _, err = isc.Update(target); err != nil {
					return err
				}

				fmt.Fprintf(o.Out, "Deleted tag %s/%s.\n", o.destNamespace[i], destNameAndTag)
				return nil
			}

			// The user wants to symlink a tag.
			istag := &imagev1.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Name:      destNameAndTag,
					Namespace: o.destNamespace[i],
				},
				Tag: &imagev1.TagReference{
					Reference: o.referenceTag,
					ImportPolicy: imagev1.TagImportPolicy{
						Insecure:  o.insecureTag,
						Scheduled: o.scheduleTag,
					},
					ReferencePolicy: imagev1.TagReferencePolicy{
						Type: tagReferencePolicy,
					},
					From: &corev1.ObjectReference{
						Kind: o.sourceKind,
					},
				},
			}
			localRef := o.ref
			switch o.sourceKind {
			case "DockerImage":
				istag.Tag.From.Name = imageutil.DockerImageReferenceExact(localRef)
				gen := int64(0)
				istag.Tag.Generation = &gen

			default:
				istag.Tag.From.Name = imageutil.DockerImageReferenceNameString(localRef)
				istag.Tag.From.Namespace = o.ref.Namespace
				if len(o.ref.Namespace) == 0 && o.destNamespace[i] != o.namespace {
					istag.Tag.From.Namespace = o.namespace
				}
			}

			msg := ""
			sameNamespace := o.namespace == o.destNamespace[i]
			if o.aliasTag {
				if sameNamespace {
					msg = fmt.Sprintf("Tag %s set up to track %s.", destNameAndTag, imageutil.DockerImageReferenceExact(o.ref))
				} else {
					msg = fmt.Sprintf("Tag %s/%s set up to track %s.", o.destNamespace[i], destNameAndTag, imageutil.DockerImageReferenceExact(o.ref))
				}
			} else {
				if istag.Tag.ImportPolicy.Scheduled {
					if sameNamespace {
						msg = fmt.Sprintf("Tag %s set to import %s periodically.", destNameAndTag, imageutil.DockerImageReferenceExact(o.ref))
					} else {
						msg = fmt.Sprintf("Tag %s/%s set to %s periodically.", o.destNamespace[i], destNameAndTag, imageutil.DockerImageReferenceExact(o.ref))
					}
				} else {
					if sameNamespace {
						msg = fmt.Sprintf("Tag %s set to %s.", destNameAndTag, imageutil.DockerImageReferenceExact(o.ref))
					} else {
						msg = fmt.Sprintf("Tag %s/%s set to %s.", o.destNamespace[i], destNameAndTag, imageutil.DockerImageReferenceExact(o.ref))
					}
				}
			}

			// supported by new servers.
			_, err := o.client.ImageStreamTags(o.destNamespace[i]).Update(istag)
			switch {
			case err == nil:
				fmt.Fprintln(o.Out, msg)
				return nil

			case kerrors.IsMethodNotSupported(err), kerrors.IsForbidden(err), kerrors.IsNotFound(err):
				// if we got one of these errors, it possible that a Create will do what we need.  Try that
				_, err := o.client.ImageStreamTags(o.destNamespace[i]).Create(istag)
				switch {
				case err == nil:
					fmt.Fprintln(o.Out, msg)
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

			target, err := isc.Get(destName, metav1.GetOptions{})
			if kerrors.IsNotFound(err) {
				target = &imagev1.ImageStream{
					ObjectMeta: metav1.ObjectMeta{
						Name: destName,
					},
				}
			} else if err != nil {
				return err
			}

			if target.Spec.Tags == nil {
				target.Spec.Tags = []imagev1.TagReference{}
			}

			if oldTargetTag, exists := imageutil.SpecHasTag(target, destTag); exists {
				if oldTargetTag.Generation == nil {
					// for servers that do not support tag generations, we need to force re-import to fetch its metadata
					delete(target.Annotations, imageapi.DockerImageRepositoryCheckAnnotation)
					istag.Tag.Generation = nil
				}
			}
			// update tag
			for i := range target.Spec.Tags {
				t := target.Spec.Tags[i]
				if t.Name == destTag {
					t = *istag.Tag
					break
				}
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

			fmt.Fprintln(o.Out, msg)
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}
