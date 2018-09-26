package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/golang/glog"
	digest "github.com/opencontainers/go-digest"
	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	imageapi "github.com/openshift/api/image/v1"
	"github.com/openshift/origin/pkg/image/apis/image/docker10"
	imagereference "github.com/openshift/origin/pkg/image/apis/image/reference"
	"github.com/openshift/origin/pkg/oc/cli/image/extract"
	"github.com/openshift/origin/pkg/oc/cli/image/mirror"
)

// NewMirrorOptions creates the options for mirroring a release.
func NewMirrorOptions(streams genericclioptions.IOStreams) *MirrorOptions {
	return &MirrorOptions{
		IOStreams: streams,
	}
}

// NewMirror creates a command to mirror an existing release.
//
// Example command to mirror a release to a local repository to work offline
//
// $ oc adm release mirror \
//     --from=registry.svc.ci.openshift.org/openshift/v4.0 \
//     --to=mycompany.com/myrepository/repo
//
// Example command to mirror and promote a release (tooling focused)
//
// $ oc adm release mirror \
//     --from=registry.svc.ci.openshift.org/openshift/v4.0-20180926095350 \
//     '--to=quay.io/openshift-test-dev/origin-v4.0:v4.1.2-${component}' \
//     --to-release-image quay.io/openshift-test-dev/origin-release:v4.1.2 \
//     --rewrite
//
func NewMirror(f kcmdutil.Factory, parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewMirrorOptions(streams)
	cmd := &cobra.Command{
		Use:   "mirror",
		Short: "Mirror a release to a different image registry location",
		Long: templates.LongDesc(`
			Copies the images and update payload for a given release from one registry to another.
			By default this command will not alter the payload and will print out the configuration
			that must be applied to a cluster to use the mirror, but you may opt to rewrite the
			update to point to the new location and lose the cryptographic integrity of the update.

			The common use for this command is to mirror a specific OpenShift release version to
			a private registry for use in a disconnected or offline context. The command copies all
			images that are part of a release into the target repository and then prints the
			correct information to give to OpenShift to use that content offline.

			Experimental: This command is under active development and may change without notice.
		`),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&o.From, "from", o.From, "Image containing the release payload.")
	flags.StringVar(&o.To, "to", o.To, "An image repository to push to.")

	flags.StringVar(&o.ToRelease, "to-release-image", o.ToRelease, "Specify one or more alternate locations for the release image instead as tag 'release' in --to")
	flags.BoolVar(&o.SkipRelease, "skip-release-image", o.SkipRelease, "Do not push the release image.")
	flags.BoolVar(&o.Rewrite, "rewrite", o.Rewrite, "Update the release image with the new image locations.")
	flags.StringVar(&o.Rename, "rename", o.Rename, "Change the name of the release - implies --rewrite.")
	return cmd
}

type MirrorOptions struct {
	genericclioptions.IOStreams

	From string

	To        string
	ToRelease string

	Rename      string
	Rewrite     bool
	SkipRelease bool
}

func (o *MirrorOptions) Complete(cmd *cobra.Command, args []string) error {
	return nil
}

const replaceComponentMarker = "X-X-X-X-X-X-X"

func (o *MirrorOptions) Run() error {
	if len(o.From) == 0 {
		return fmt.Errorf("must specify an image containing a release payload with --from")
	}

	if len(o.To) == 0 {
		return fmt.Errorf("must specify an image repository to mirror the release to")
	}

	if o.SkipRelease && len(o.ToRelease) > 0 {
		return fmt.Errorf("--skip-release-image and --to-release-image may not both be specified")
	}

	if len(o.Rename) > 0 {
		o.Rewrite = true
	}

	src := o.From
	srcRef, err := imagereference.Parse(src)
	if err != nil {
		return err
	}
	var recreateRequired bool
	var targetFn func(name string) imagereference.DockerImageReference
	dst := o.To
	if strings.Contains(dst, "${component}") {
		format := strings.Replace(dst, "${component}", replaceComponentMarker, -1)
		dstRef, err := imagereference.Parse(format)
		if err != nil {
			return fmt.Errorf("--to must be a valid image reference: %v", err)
		}
		targetFn = func(name string) imagereference.DockerImageReference {
			value := strings.Replace(dst, "${component}", name, -1)
			ref, err := imagereference.Parse(value)
			if err != nil {
				glog.Fatalf("requested component %q could not be injected into %s: %v", name, dst, err)
			}
			return ref
		}
		replaceCount := strings.Count(dst, "${component}")
		recreateRequired = replaceCount > 1 || (replaceCount == 1 && !strings.Contains(dstRef.Tag, replaceComponentMarker))

	} else {
		ref, err := imagereference.Parse(dst)
		if err != nil {
			return fmt.Errorf("--to must be a valid image repository: %v", err)
		}
		if len(ref.ID) > 0 || len(ref.Tag) > 0 {
			return fmt.Errorf("--to must be to an image repository and may not contain a tag or digest")
		}
		targetFn = func(name string) imagereference.DockerImageReference {
			copied := ref
			copied.Tag = name
			return copied
		}
	}

	if recreateRequired && !o.Rewrite {
		return fmt.Errorf("when mirroring to multiple repositories, rewriting the release is required")
	}

	var is *imageapi.ImageStream
	var fromDigest digest.Digest

	// load image references
	buf := &bytes.Buffer{}
	extractOpts := NewExtractOptions(genericclioptions.IOStreams{Out: buf, ErrOut: o.ErrOut})
	extractOpts.ImageMetadataCallback = func(m *extract.Mapping, dgst digest.Digest, config *docker10.DockerImageConfig) {
		fromDigest = dgst
	}
	extractOpts.From = o.From
	extractOpts.File = "image-references"
	if err := extractOpts.Run(); err != nil {
		return fmt.Errorf("unable to retrieve release image info: %v", err)
	}
	if err := json.Unmarshal(buf.Bytes(), &is); err != nil {
		return fmt.Errorf("unable to load image-references from release payload: %v", err)
	}
	if is.Kind != "ImageStream" || is.APIVersion != "image.openshift.io/v1" {
		return fmt.Errorf("unrecognized image-references in release payload")
	}

	// build the mapping list for mirroring and rewrite if necessary
	var mappings []mirror.Mapping
	for i := range is.Spec.Tags {
		tag := &is.Spec.Tags[i]
		if tag.From == nil || tag.From.Kind != "DockerImage" {
			continue
		}
		from, err := imagereference.Parse(tag.From.Name)
		if err != nil {
			return fmt.Errorf("release tag %q is not valid: %v", tag.Name, err)
		}
		mappings = append(mappings, mirror.Mapping{
			Type:        mirror.DestinationRegistry,
			Source:      from,
			Destination: targetFn(tag.Name),
		})
		glog.V(2).Infof("Mapping %#v", mappings[len(mappings)-1])

		if o.Rewrite {
			if len(from.Tag) > 0 || len(from.ID) == 0 {
				return fmt.Errorf("image-references should only contain pointers to images by digest: %s", tag.From.Name)
			}
			dstRef := targetFn(tag.Name)
			dstRef.Tag = ""
			dstRef.ID = from.ID
			tag.From.Name = dstRef.Exact()
		}
	}

	if len(mappings) == 0 {
		fmt.Fprintf(o.ErrOut, "warning: Release image contains no image references - is this a valid release?\n")
	}

	if len(o.ToRelease) > 0 {
		dstRef, err := imagereference.Parse(o.ToRelease)
		if err != nil {
			return fmt.Errorf("invalid --to-release-image: %v", err)
		}
		mappings = append(mappings, mirror.Mapping{
			Type:        mirror.DestinationRegistry,
			Source:      srcRef,
			Destination: dstRef,
		})
	} else if !o.SkipRelease {
		mappings = append(mappings, mirror.Mapping{
			Type:        mirror.DestinationRegistry,
			Source:      srcRef,
			Destination: targetFn("release"),
		})
	}

	fmt.Fprintf(os.Stderr, "info: Mirroring %d images to %s ...\n", len(mappings), dst)
	opts := mirror.NewMirrorImageOptions(genericclioptions.IOStreams{Out: o.Out, ErrOut: o.ErrOut})
	opts.Mappings = mappings
	if err := opts.Run(); err != nil {
		return err
	}

	if o.Rewrite {
		glog.V(2).Infof("Building a release from %s", o.From)

		if is.Annotations == nil {
			is.Annotations = make(map[string]string)
		}
		is.Annotations["release.openshift.io/mirror-from"] = o.From
		is.Annotations["release.openshift.io/mirror-from-digest"] = fromDigest.String()

		newOpts := NewNewOptions(genericclioptions.IOStreams{Out: o.Out, ErrOut: o.ErrOut})
		newOpts.Name = is.Name
		if len(o.Rename) > 0 {
			newOpts.Name = o.Rename
		}
		newOpts.FromImageStreamObject = is
		newOpts.ToImageBase = targetFn("cluster-version-operator").Exact()
		if len(o.ToRelease) > 0 {
			newOpts.ToImage = o.ToRelease
		} else {
			newOpts.ToImage = targetFn("release").Exact()
		}
		if err := newOpts.Run(); err != nil {
			return err
		}
		fmt.Fprintf(o.Out, "\nSuccess\nUpdate image:  %s\nMirror prefix: %s\n", newOpts.ToImage, o.To)
		return nil
	}

	fmt.Fprintf(o.Out, "\nSuccess\nUpdate image:  %s\nMirror prefix: %s\n", targetFn("release").String(), o.To)
	return nil
}
