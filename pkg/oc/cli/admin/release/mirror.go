package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

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
func NewMirror(f kcmdutil.Factory, parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewMirrorOptions(streams)
	cmd := &cobra.Command{
		Use:   "mirror",
		Short: "Mirror a release to a different image registry location",
		Long: templates.LongDesc(`
			Mirror an OpenShift release image to another registry

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
	flags.BoolVar(&o.DryRun, "dry-run", o.DryRun, "Display information about the mirror without actually executing it.")

	flags.BoolVar(&o.SkipRelease, "skip-release-image", o.SkipRelease, "Do not push the release image.")
	flags.StringVar(&o.ToRelease, "to-release-image", o.ToRelease, "Specify an alternate locations for the release image instead as tag 'release' in --to")
	return cmd
}

type MirrorOptions struct {
	genericclioptions.IOStreams

	From string

	To          string
	ToRelease   string
	SkipRelease bool

	DryRun bool

	ImageStream *imageapi.ImageStream
	TargetFn    func(component string) imagereference.DockerImageReference
}

func (o *MirrorOptions) Complete(cmd *cobra.Command, args []string) error {
	return nil
}

const replaceComponentMarker = "X-X-X-X-X-X-X"

func (o *MirrorOptions) Run() error {
	if len(o.From) == 0 && o.ImageStream == nil {
		return fmt.Errorf("must specify an image containing a release payload with --from")
	}

	if len(o.To) == 0 {
		return fmt.Errorf("must specify an image repository to mirror the release to")
	}

	if o.SkipRelease && len(o.ToRelease) > 0 {
		return fmt.Errorf("--skip-release-image and --to-release-image may not both be specified")
	}

	var recreateRequired bool
	var hasPrefix bool
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
		hasPrefix = true
	}

	o.TargetFn = targetFn

	if recreateRequired {
		return fmt.Errorf("when mirroring to multiple repositories, use the new release command with --from-release and --mirror")
	}

	is := o.ImageStream
	if is == nil {
		o.ImageStream = &imageapi.ImageStream{}
		is = o.ImageStream
		// load image references
		buf := &bytes.Buffer{}
		extractOpts := NewExtractOptions(genericclioptions.IOStreams{Out: buf, ErrOut: o.ErrOut})
		extractOpts.ImageMetadataCallback = func(m *extract.Mapping, dgst digest.Digest, config *docker10.DockerImageConfig) {}
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
	}

	var mappings []mirror.Mapping
	if len(o.From) > 0 && !o.SkipRelease {
		src := o.From
		srcRef, err := imagereference.Parse(src)
		if err != nil {
			return err
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
	}

	// build the mapping list for mirroring and rewrite if necessary
	for i := range is.Spec.Tags {
		tag := &is.Spec.Tags[i]
		if tag.From == nil || tag.From.Kind != "DockerImage" {
			continue
		}
		from, err := imagereference.Parse(tag.From.Name)
		if err != nil {
			return fmt.Errorf("release tag %q is not valid: %v", tag.Name, err)
		}
		if len(from.Tag) > 0 || len(from.ID) == 0 {
			return fmt.Errorf("image-references should only contain pointers to images by digest: %s", tag.From.Name)
		}

		mappings = append(mappings, mirror.Mapping{
			Type:        mirror.DestinationRegistry,
			Source:      from,
			Destination: targetFn(tag.Name),
		})
		glog.V(2).Infof("Mapping %#v", mappings[len(mappings)-1])

		dstRef := targetFn(tag.Name)
		dstRef.Tag = ""
		dstRef.ID = from.ID
		tag.From.Name = dstRef.Exact()
	}

	if len(mappings) == 0 {
		fmt.Fprintf(o.ErrOut, "warning: Release image contains no image references - is this a valid release?\n")
	}

	fmt.Fprintf(os.Stderr, "info: Mirroring %d images to %s ...\n", len(mappings), dst)
	var lock sync.Mutex
	opts := mirror.NewMirrorImageOptions(genericclioptions.IOStreams{Out: o.Out, ErrOut: o.ErrOut})
	opts.Mappings = mappings
	opts.DryRun = o.DryRun
	opts.ManifestUpdateCallback = func(registry string, manifests map[digest.Digest]digest.Digest) error {
		lock.Lock()
		defer lock.Unlock()

		// when uploading to a schema1 registry, manifest ids change and we must remap them
		for i := range is.Spec.Tags {
			tag := &is.Spec.Tags[i]
			if tag.From == nil || tag.From.Kind != "DockerImage" {
				continue
			}
			ref, err := imagereference.Parse(tag.From.Name)
			if err != nil {
				return fmt.Errorf("unable to parse image reference %s (%s): %v", tag.Name, tag.From.Name, err)
			}
			if ref.Registry != registry {
				continue
			}
			if changed, ok := manifests[digest.Digest(ref.ID)]; ok {
				ref.ID = changed.String()
				glog.V(4).Infof("During mirroring, image %s was updated to digest %s", tag.From.Name, changed)
				tag.From.Name = ref.Exact()
			}
		}
		return nil
	}
	if err := opts.Run(); err != nil {
		return err
	}

	to := o.ToRelease
	if len(to) == 0 {
		to = targetFn("release").String()
	}
	if hasPrefix {
		fmt.Fprintf(o.Out, "\nSuccess\nUpdate image:  %s\nMirror prefix: %s\n", to, o.To)
	} else {
		fmt.Fprintf(o.Out, "\nSuccess\nUpdate image:  %s\nMirrored to: %s\n", to, o.To)
	}
	return nil
}

func sourceImageRef(is *imageapi.ImageStream, name string) (string, bool) {
	for _, tag := range is.Spec.Tags {
		if tag.Name != name {
			continue
		}
		if tag.From == nil || tag.From.Kind != "DockerImage" {
			return "", false
		}
		return tag.From.Name, true
	}
	return "", false
}
