package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	imageapi "github.com/openshift/api/image/v1"
	imagereference "github.com/openshift/origin/pkg/image/apis/image/reference"
	"github.com/openshift/origin/pkg/oc/cli/image/mirror"
)

func NewMirrorOptions(streams genericclioptions.IOStreams) *MirrorOptions {
	return &MirrorOptions{
		IOStreams: streams,
	}
}

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
	return cmd
}

type MirrorOptions struct {
	genericclioptions.IOStreams

	From string

	To string
}

func (o *MirrorOptions) Complete(cmd *cobra.Command, args []string) error {
	return nil
}

func (o *MirrorOptions) Run() error {
	if len(o.From) == 0 {
		return fmt.Errorf("must specify an image containing a release payload with --from")
	}

	if len(o.To) == 0 {
		return fmt.Errorf("must specify an image repository to mirror the release to")
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
		format := strings.Replace(dst, "${component}", "test", -1)
		_, err := imagereference.Parse(format)
		if err != nil {
			return fmt.Errorf("--to must be a valid image reference: %v", err)
		}
		targetFn = func(name string) imagereference.DockerImageReference {
			ref, err := imagereference.Parse(strings.Replace(dst, "${component}", name, -1))
			if err != nil {
				glog.Fatalf("provided component %q is not a valid image reference: %v", name, err)
			}
			return ref
		}
		recreateRequired = strings.Count(dst, "${component}") > 1 || strings.HasSuffix(dst, ":${component}")

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

	if recreateRequired {
		return fmt.Errorf("not implemented: changing release URLs is not currently supported")
	}

	buf := &bytes.Buffer{}
	extractOpts := NewExtractOptions(genericclioptions.IOStreams{Out: buf, ErrOut: o.ErrOut})
	extractOpts.From = o.From
	extractOpts.File = "image-references"
	if err := extractOpts.Run(); err != nil {
		return fmt.Errorf("unable to retrieve release image info: %v", err)
	}
	is := &imageapi.ImageStream{}
	if err := json.Unmarshal(buf.Bytes(), &is); err != nil {
		return fmt.Errorf("unable to load image-references from release payload: %v", err)
	}
	if is.Kind != "ImageStream" || is.APIVersion != "image.openshift.io/v1" {
		return fmt.Errorf("unrecognized image-references in release payload")
	}
	var mappings []mirror.Mapping
	for _, tag := range is.Spec.Tags {
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
	}
	if len(mappings) == 0 {
		fmt.Fprintf(o.ErrOut, "warning: Release image contains no image references - is this a valid release?\n")
	}
	mappings = append(mappings, mirror.Mapping{
		Type:        mirror.DestinationRegistry,
		Source:      srcRef,
		Destination: targetFn("release"),
	})

	fmt.Fprintf(os.Stderr, "info: Mirroring %d images to %s ...\n", len(mappings), targetFn("${component}"))

	opts := mirror.NewMirrorImageOptions(genericclioptions.IOStreams{Out: o.Out, ErrOut: o.ErrOut})
	opts.Mappings = mappings
	if err := opts.Run(); err != nil {
		return err
	}
	fmt.Fprintf(o.Out, "\nSuccess\nUpdate image:  %s\nMirror prefix: %s\n", targetFn("release").String(), o.To)
	return nil
}
