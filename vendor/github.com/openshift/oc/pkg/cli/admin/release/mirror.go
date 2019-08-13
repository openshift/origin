package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	digest "github.com/opencontainers/go-digest"
	"github.com/spf13/cobra"
	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/util/retry"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	imagev1 "github.com/openshift/api/image/v1"
	imageclient "github.com/openshift/client-go/image/clientset/versioned"
	"github.com/openshift/library-go/pkg/image/dockerv1client"
	imagereference "github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/oc/pkg/cli/image/extract"
	imagemanifest "github.com/openshift/oc/pkg/cli/image/manifest"
	"github.com/openshift/oc/pkg/cli/image/mirror"
)

// NewMirrorOptions creates the options for mirroring a release.
func NewMirrorOptions(streams genericclioptions.IOStreams) *MirrorOptions {
	return &MirrorOptions{
		IOStreams:       streams,
		ParallelOptions: imagemanifest.ParallelOptions{MaxPerRegistry: 6},
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
			correct information to give to OpenShift to use that content offline. An alternate mode
			is to specify --to-image-stream, which imports the images directly into an OpenShift
			image stream.
		`),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(cmd, f, args))
			kcmdutil.CheckErr(o.Run())
		},
	}
	flags := cmd.Flags()
	o.SecurityOptions.Bind(flags)
	o.ParallelOptions.Bind(flags)

	flags.StringVar(&o.From, "from", o.From, "Image containing the release payload.")
	flags.StringVar(&o.To, "to", o.To, "An image repository to push to.")
	flags.StringVar(&o.ToImageStream, "to-image-stream", o.ToImageStream, "An image stream to tag images into.")
	flags.BoolVar(&o.DryRun, "dry-run", o.DryRun, "Display information about the mirror without actually executing it.")

	flags.BoolVar(&o.SkipRelease, "skip-release-image", o.SkipRelease, "Do not push the release image.")
	flags.StringVar(&o.ToRelease, "to-release-image", o.ToRelease, "Specify an alternate locations for the release image instead as tag 'release' in --to")
	return cmd
}

type MirrorOptions struct {
	genericclioptions.IOStreams

	SecurityOptions imagemanifest.SecurityOptions
	ParallelOptions imagemanifest.ParallelOptions

	From string

	To            string
	ToImageStream string

	ToRelease   string
	SkipRelease bool

	DryRun bool

	ClientFn func() (imageclient.Interface, string, error)

	ImageStream *imagev1.ImageStream
	TargetFn    func(component string) imagereference.DockerImageReference
}

func (o *MirrorOptions) Complete(cmd *cobra.Command, f kcmdutil.Factory, args []string) error {
	switch {
	case len(args) == 0 && len(o.From) == 0:
		return fmt.Errorf("must specify a release image with --from")
	case len(args) == 1 && len(o.From) == 0:
		o.From = args[0]
	case len(args) == 1 && len(o.From) > 0:
		return fmt.Errorf("you may not specify an argument and --from")
	case len(args) > 1:
		return fmt.Errorf("only one argument is accepted")
	}
	o.ClientFn = func() (imageclient.Interface, string, error) {
		cfg, err := f.ToRESTConfig()
		if err != nil {
			return nil, "", err
		}
		client, err := imageclient.NewForConfig(cfg)
		if err != nil {
			return nil, "", err
		}
		ns, _, err := f.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return nil, "", err
		}
		return client, ns, nil
	}
	return nil
}

const replaceComponentMarker = "X-X-X-X-X-X-X"

func (o *MirrorOptions) Run() error {
	if len(o.From) == 0 && o.ImageStream == nil {
		return fmt.Errorf("must specify a release image with --from")
	}

	if (len(o.To) == 0) == (len(o.ToImageStream) == 0) {
		return fmt.Errorf("must specify an image repository or image stream to mirror the release to")
	}

	if o.SkipRelease && len(o.ToRelease) > 0 {
		return fmt.Errorf("--skip-release-image and --to-release-image may not both be specified")
	}

	var recreateRequired bool
	var hasPrefix bool
	var targetFn func(name string) mirror.MirrorReference
	var dst string
	if len(o.ToImageStream) > 0 {
		dst = imagereference.DockerImageReference{
			Registry:  "example.com",
			Namespace: "somenamespace",
			Name:      "mirror",
		}.Exact()
	} else {
		dst = o.To
	}

	if strings.Contains(dst, "${component}") {
		format := strings.Replace(dst, "${component}", replaceComponentMarker, -1)
		dstRef, err := mirror.ParseMirrorReference(format)
		if err != nil {
			return fmt.Errorf("--to must be a valid image reference: %v", err)
		}
		targetFn = func(name string) mirror.MirrorReference {
			value := strings.Replace(dst, "${component}", name, -1)
			ref, err := mirror.ParseMirrorReference(value)
			if err != nil {
				klog.Fatalf("requested component %q could not be injected into %s: %v", name, dst, err)
			}
			return ref
		}
		replaceCount := strings.Count(dst, "${component}")
		recreateRequired = replaceCount > 1 || (replaceCount == 1 && !strings.Contains(dstRef.Tag, replaceComponentMarker))

	} else {
		ref, err := mirror.ParseMirrorReference(dst)
		if err != nil {
			return fmt.Errorf("--to must be a valid image repository: %v", err)
		}
		if len(ref.ID) > 0 || len(ref.Tag) > 0 {
			return fmt.Errorf("--to must be to an image repository and may not contain a tag or digest")
		}
		targetFn = func(name string) mirror.MirrorReference {
			copied := ref
			copied.Tag = name
			return copied
		}
		hasPrefix = true
	}

	o.TargetFn = func(name string) imagereference.DockerImageReference {
		ref := targetFn(name)
		return ref.DockerImageReference
	}

	if recreateRequired {
		return fmt.Errorf("when mirroring to multiple repositories, use the new release command with --from-release and --mirror")
	}

	verifier := imagemanifest.NewVerifier()
	is := o.ImageStream
	if is == nil {
		o.ImageStream = &imagev1.ImageStream{}
		is = o.ImageStream
		// load image references
		buf := &bytes.Buffer{}
		extractOpts := NewExtractOptions(genericclioptions.IOStreams{Out: buf, ErrOut: o.ErrOut})
		extractOpts.SecurityOptions = o.SecurityOptions
		extractOpts.ImageMetadataCallback = func(m *extract.Mapping, dgst, contentDigest digest.Digest, config *dockerv1client.DockerImageConfig) {
			verifier.Verify(dgst, contentDigest)
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
		if !verifier.Verified() {
			err := fmt.Errorf("the release image failed content verification and may have been tampered with")
			if !o.SecurityOptions.SkipVerification {
				return err
			}
			fmt.Fprintf(o.ErrOut, "warning: %v\n", err)
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
				Name:        o.ToRelease,
			})
		} else if !o.SkipRelease {
			dstRef := targetFn("release")
			mappings = append(mappings, mirror.Mapping{
				Source:      srcRef,
				Type:        dstRef.Type(),
				Destination: dstRef.Combined(),
				Name:        "release",
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

		dstMirrorRef := targetFn(tag.Name)
		mappings = append(mappings, mirror.Mapping{
			Source:      from,
			Type:        dstMirrorRef.Type(),
			Destination: dstMirrorRef.Combined(),
			Name:        tag.Name,
		})
		klog.V(2).Infof("Mapping %#v", mappings[len(mappings)-1])

		dstRef := targetFn(tag.Name)
		dstRef.Tag = ""
		dstRef.ID = from.ID
		tag.From.Name = dstRef.Exact()
	}

	if len(mappings) == 0 {
		fmt.Fprintf(o.ErrOut, "warning: Release image contains no image references - is this a valid release?\n")
	}

	if len(o.ToImageStream) > 0 {
		remaining := make(map[string]mirror.Mapping)
		for _, mapping := range mappings {
			remaining[mapping.Name] = mapping
		}
		client, ns, err := o.ClientFn()
		if err != nil {
			return err
		}
		hasErrors := make(map[string]error)
		maxPerIteration := 12

		for retries := 4; (len(remaining) > 0 || len(hasErrors) > 0) && retries > 0; {
			if len(remaining) == 0 {
				for _, mapping := range mappings {
					if _, ok := hasErrors[mapping.Name]; ok {
						remaining[mapping.Name] = mapping
						delete(hasErrors, mapping.Name)
					}
				}
				retries--
			}
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				isi := &imagev1.ImageStreamImport{
					ObjectMeta: metav1.ObjectMeta{
						Name: o.ToImageStream,
					},
					Spec: imagev1.ImageStreamImportSpec{
						Import: !o.DryRun,
					},
				}
				for _, mapping := range remaining {
					isi.Spec.Images = append(isi.Spec.Images, imagev1.ImageImportSpec{
						From: corev1.ObjectReference{
							Kind: "DockerImage",
							Name: mapping.Source.Exact(),
						},
						To: &corev1.LocalObjectReference{
							Name: mapping.Name,
						},
					})
					if len(isi.Spec.Images) > maxPerIteration {
						break
					}
				}

				// use RESTClient directly here to be able to extend request timeout
				result := &imagev1.ImageStreamImport{}
				if err := client.ImageV1().RESTClient().Post().
					Namespace(ns).
					Resource(imagev1.Resource("imagestreamimports").Resource).
					Body(isi).
					// this instructs the api server to allow our request to take up to an hour - chosen as a high boundary
					Timeout(3 * time.Minute).
					Do().
					Into(result); err != nil {
					return err
				}

				for i, image := range result.Status.Images {
					name := result.Spec.Images[i].To.Name
					klog.V(4).Infof("Import result for %s: %#v", name, image.Status)
					if image.Status.Status == metav1.StatusSuccess {
						delete(remaining, name)
						delete(hasErrors, name)
					} else {
						delete(remaining, name)
						err := errors.FromObject(&image.Status)
						hasErrors[name] = err
						klog.V(2).Infof("Failed to import %s as tag %s: %v", remaining[name].Source, name, err)
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
		}

		if len(hasErrors) > 0 {
			var messages []string
			for k, v := range hasErrors {
				messages = append(messages, fmt.Sprintf("%s: %v", k, v))
			}
			sort.Strings(messages)
			if len(messages) == 1 {
				return fmt.Errorf("unable to import a release image: %s", messages[0])
			}
			return fmt.Errorf("unable to import some release images:\n* %s", strings.Join(messages, "\n* "))
		}

		fmt.Fprintf(os.Stderr, "Mirrored %d images to %s/%s\n", len(mappings), ns, o.ToImageStream)
		return nil
	}

	fmt.Fprintf(os.Stderr, "info: Mirroring %d images to %s ...\n", len(mappings), dst)
	var lock sync.Mutex
	opts := mirror.NewMirrorImageOptions(genericclioptions.IOStreams{Out: o.Out, ErrOut: o.ErrOut})
	opts.SecurityOptions = o.SecurityOptions
	opts.ParallelOptions = o.ParallelOptions
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
				klog.V(4).Infof("During mirroring, image %s was updated to digest %s", tag.From.Name, changed)
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

func sourceImageRef(is *imagev1.ImageStream, name string) (string, bool) {
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
