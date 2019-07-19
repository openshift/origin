package info

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	units "github.com/docker/go-units"
	digest "github.com/opencontainers/go-digest"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	"github.com/openshift/library-go/pkg/image/dockerv1client"
	imagereference "github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/library-go/pkg/image/registryclient"
	imagemanifest "github.com/openshift/oc/pkg/cli/image/manifest"
	"github.com/openshift/oc/pkg/cli/image/workqueue"
)

func NewInfoOptions(streams genericclioptions.IOStreams) *InfoOptions {
	return &InfoOptions{
		IOStreams: streams,
	}
}

func NewInfo(parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewInfoOptions(streams)
	cmd := &cobra.Command{
		Use:   "info IMAGE",
		Short: "Display information about an image",
		Long: templates.LongDesc(`
			Show information about an image in a remote image registry

			Experimental: This command is under active development and may change without notice.
		`),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}
	flags := cmd.Flags()
	o.FilterOptions.Bind(flags)
	o.SecurityOptions.Bind(flags)
	flags.StringVarP(&o.Output, "output", "o", o.Output, "Print the image in an alternative format: json")
	return cmd
}

type InfoOptions struct {
	genericclioptions.IOStreams

	SecurityOptions imagemanifest.SecurityOptions
	FilterOptions   imagemanifest.FilterOptions

	Images []string

	Output string
}

func (o *InfoOptions) Complete(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("info expects at least one argument, an image pull spec")
	}
	o.Images = args
	return nil
}

func (o *InfoOptions) Validate() error {
	return o.FilterOptions.Validate()
}

func (o *InfoOptions) Run() error {
	if len(o.Images) == 0 {
		return fmt.Errorf("must specify one or more images as arguments")
	}

	// cache the context
	_, err := o.SecurityOptions.Context()
	if err != nil {
		return err
	}

	hadError := false
	for _, location := range o.Images {
		src, err := imagereference.Parse(location)
		if err != nil {
			return err
		}
		if len(src.Tag) == 0 && len(src.ID) == 0 {
			return fmt.Errorf("--from must point to an image ID or image tag")
		}

		var image *Image
		retriever := &ImageRetriever{
			Image: map[string]imagereference.DockerImageReference{
				location: src,
			},
			SecurityOptions: o.SecurityOptions,
			ManifestListCallback: func(from string, list *manifestlist.DeserializedManifestList, all map[digest.Digest]distribution.Manifest) (map[digest.Digest]distribution.Manifest, error) {
				filtered := make(map[digest.Digest]distribution.Manifest)
				for _, manifest := range list.Manifests {
					if !o.FilterOptions.Include(&manifest, len(list.Manifests) > 1) {
						klog.V(5).Infof("Skipping image for %#v from %s", manifest.Platform, from)
						continue
					}
					filtered[manifest.Digest] = all[manifest.Digest]
				}
				if len(filtered) == 1 {
					return filtered, nil
				}

				buf := &bytes.Buffer{}
				w := tabwriter.NewWriter(buf, 0, 0, 1, ' ', 0)
				fmt.Fprintf(w, "  OS\tDIGEST\n")
				for _, manifest := range list.Manifests {
					fmt.Fprintf(w, "  %s\t%s\n", imagemanifest.PlatformSpecString(manifest.Platform), manifest.Digest)
				}
				w.Flush()
				return nil, fmt.Errorf("the image is a manifest list and contains multiple images - use --filter-by-os to select from:\n\n%s\n", buf.String())
			},

			ImageMetadataCallback: func(from string, i *Image, err error) error {
				if err != nil {
					return err
				}
				image = i
				return nil
			},
		}
		if err := retriever.Run(); err != nil {
			return err
		}

		switch o.Output {
		case "":
		case "json":
			data, err := json.MarshalIndent(image, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintf(o.Out, "%s", string(data))
			continue
		default:
			return fmt.Errorf("unrecognized --output, only 'json' is supported")
		}

		if err := describeImage(o.Out, image); err != nil {
			hadError = true
			if err != kcmdutil.ErrExit {
				fmt.Fprintf(o.ErrOut, "error: %v", err)
			}
		}

	}
	if hadError {
		return kcmdutil.ErrExit
	}
	return nil
}

type Image struct {
	Name          string                              `json:"name"`
	Ref           imagereference.DockerImageReference `json:"-"`
	Digest        digest.Digest                       `json:"digest"`
	ContentDigest digest.Digest                       `json:"contentDigest"`
	ListDigest    digest.Digest                       `json:"listDigest"`
	MediaType     string                              `json:"mediaType"`
	Layers        []distribution.Descriptor           `json:"layers"`
	Config        *dockerv1client.DockerImageConfig   `json:"config"`

	Manifest distribution.Manifest `json:"-"`
}

func describeImage(out io.Writer, image *Image) error {
	var err error

	w := tabwriter.NewWriter(out, 0, 4, 1, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "Name:\t%s\n", image.Name)
	if len(image.Ref.ID) == 0 || image.Ref.ID != image.Digest.String() {
		fmt.Fprintf(w, "Digest:\t%s\n", image.Digest)
	}
	if len(image.ListDigest) > 0 {
		fmt.Fprintf(w, "Manifest List:\t%s\n", image.ListDigest)
	}
	if image.ContentDigest != image.Digest {
		fmt.Fprintf(w, "Content Digest:\t%s\n\tERROR: the image contents do not match the requested digest, this image has been tampered with\n", image.ContentDigest)
		err = kcmdutil.ErrExit
	}

	fmt.Fprintf(w, "Media Type:\t%s\n", image.MediaType)
	if image.Config.Created.IsZero() {
		fmt.Fprintf(w, "Created:\t%s\n", "<unknown>")
	} else {
		fmt.Fprintf(w, "Created:\t%s ago\n", duration.ShortHumanDuration(time.Now().Sub(image.Config.Created)))
	}
	switch l := len(image.Layers); l {
	case 0:
		// legacy case, server does not know individual layers
		fmt.Fprintf(w, "Layer Size:\t%s\n", units.HumanSize(float64(image.Config.Size)))
	default:
		imageSize := fmt.Sprintf("%s in %d layers", units.HumanSize(float64(image.Config.Size)), len(image.Layers))
		if image.Config.Size == 0 {
			imageSize = fmt.Sprintf("%d layers (size unavailable)", len(image.Layers))
		}

		fmt.Fprintf(w, "Image Size:\t%s\n", imageSize)
		for i, layer := range image.Layers {
			layerSize := units.HumanSize(float64(layer.Size))
			if layer.Size == 0 {
				layerSize = "--"
			}

			if i == 0 {
				fmt.Fprintf(w, "%s\t%s\t%s\n", "Layers:", layerSize, layer.Digest)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\n", "", layerSize, layer.Digest)
			}
		}
	}
	fmt.Fprintf(w, "OS:\t%s\n", image.Config.OS)
	fmt.Fprintf(w, "Arch:\t%s\n", image.Config.Architecture)
	if len(image.Config.Author) > 0 {
		fmt.Fprintf(w, "Author:\t%s\n", image.Config.Author)
	}

	config := image.Config.Config
	if config != nil {
		hasCommand := false
		if len(config.Entrypoint) > 0 {
			hasCommand = true
			fmt.Fprintf(w, "Entrypoint:\t%s\n", strings.Join(config.Entrypoint, " "))
		}
		if len(config.Cmd) > 0 {
			hasCommand = true
			fmt.Fprintf(w, "Command:\t%s\n", strings.Join(config.Cmd, " "))
		}
		if !hasCommand {
			fmt.Fprintf(w, "Command:\t%s\n", "<none>")
		}
		if len(config.WorkingDir) > 0 {
			fmt.Fprintf(w, "Working Dir:\t%s\n", config.WorkingDir)
		}
		if len(config.User) > 0 {
			fmt.Fprintf(w, "User:\t%s\n", config.User)
		}
		ports := sets.NewString()
		for k := range config.ExposedPorts {
			ports.Insert(k)
		}
		if len(ports) > 0 {
			fmt.Fprintf(w, "Exposes Ports:\t%s\n", strings.Join(ports.List(), ", "))
		}
	}

	if config != nil && len(config.Env) > 0 {
		for i, env := range config.Env {
			if i == 0 {
				fmt.Fprintf(w, "%s\t%s\n", "Environment:", env)
			} else {
				fmt.Fprintf(w, "%s\t%s\n", "", env)
			}
		}
	}

	if config != nil && len(config.Labels) > 0 {
		var keys []string
		for k := range config.Labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, key := range keys {
			if i == 0 {
				fmt.Fprintf(w, "%s\t%s=%s\n", "Labels:", key, config.Labels[key])
			} else {
				fmt.Fprintf(w, "%s\t%s=%s\n", "", key, config.Labels[key])
			}
		}
	}

	if config != nil && len(config.Volumes) > 0 {
		var keys []string
		for k := range config.Volumes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, volume := range keys {
			if i == 0 {
				fmt.Fprintf(w, "%s\t%s\n", "Volumes:", volume)
			} else {
				fmt.Fprintf(w, "%s\t%s\n", "", volume)
			}
		}
	}

	fmt.Fprintln(w)
	return err
}

func writeTabSection(out io.Writer, fn func(w io.Writer)) {
	w := tabwriter.NewWriter(out, 0, 4, 1, ' ', 0)
	fn(w)
	w.Flush()
}

type ImageRetriever struct {
	Image           map[string]imagereference.DockerImageReference
	SecurityOptions imagemanifest.SecurityOptions
	ParallelOptions imagemanifest.ParallelOptions
	// ImageMetadataCallback is invoked once per image retrieved, and may be called in parallel if
	// MaxPerRegistry is set higher than 1. If err is passed image is nil. If an error is returned
	// execution will stop.
	ImageMetadataCallback func(from string, image *Image, err error) error
	// ManifestListCallback, if specified, is invoked if the root image is a manifest list. If an
	// error returned processing stops. If zero manifests are returned the next item is rendered
	// and no ImageMetadataCallback calls occur. If more than one manifest is returned
	// ImageMetadataCallback will be invoked once for each item.
	ManifestListCallback func(from string, list *manifestlist.DeserializedManifestList, all map[digest.Digest]distribution.Manifest) (map[digest.Digest]distribution.Manifest, error)
}

func (o *ImageRetriever) Run() error {
	ctx := context.Background()
	fromContext, err := o.SecurityOptions.Context()
	if err != nil {
		return err
	}

	callbackFn := o.ImageMetadataCallback
	if callbackFn == nil {
		callbackFn = func(_ string, _ *Image, err error) error {
			return err
		}
	}
	stopCh := make(chan struct{})
	defer close(stopCh)
	q := workqueue.New(o.ParallelOptions.MaxPerRegistry, stopCh)
	return q.Try(func(q workqueue.Try) {
		for key := range o.Image {
			name := key
			from := o.Image[key]
			q.Try(func() error {
				repo, err := fromContext.Repository(ctx, from.DockerClientDefaults().RegistryURL(), from.RepositoryName(), o.SecurityOptions.Insecure)
				if err != nil {
					return callbackFn(name, nil, fmt.Errorf("unable to connect to image repository %s: %v", from.Exact(), err))
				}

				allManifests, manifestList, listDigest, err := imagemanifest.AllManifests(ctx, from, repo)
				if err != nil {
					if imagemanifest.IsImageForbidden(err) {
						var msg string
						if len(o.Image) == 1 {
							msg = "image does not exist or you don't have permission to access the repository"
						} else {
							msg = fmt.Sprintf("image %q does not exist or you don't have permission to access the repository", from)
						}
						return callbackFn(name, nil, imagemanifest.NewImageForbidden(msg, err))
					}
					if imagemanifest.IsImageNotFound(err) {
						var msg string
						if len(o.Image) == 1 {
							msg = "image does not exist"
						} else {
							msg = fmt.Sprintf("image %q does not exist", from)
						}
						return callbackFn(name, nil, imagemanifest.NewImageNotFound(msg, err))
					}
					return callbackFn(name, nil, fmt.Errorf("unable to read image %s: %v", from, err))
				}

				if o.ManifestListCallback != nil && manifestList != nil {
					allManifests, err = o.ManifestListCallback(name, manifestList, allManifests)
					if err != nil {
						return err
					}
				}

				if len(allManifests) == 0 {
					return imagemanifest.NewImageNotFound(fmt.Sprintf("no manifests could be found for %q", from), nil)
				}

				for srcDigest, srcManifest := range allManifests {
					contentDigest, contentErr := registryclient.ContentDigestForManifest(srcManifest, srcDigest.Algorithm())
					if contentErr != nil {
						return callbackFn(name, nil, contentErr)
					}

					imageConfig, layers, manifestErr := imagemanifest.ManifestToImageConfig(ctx, srcManifest, repo.Blobs(ctx), imagemanifest.ManifestLocation{ManifestList: listDigest, Manifest: srcDigest})
					mediaType, _, _ := srcManifest.Payload()
					if err := callbackFn(name, &Image{
						Name:          from.Exact(),
						Ref:           from,
						MediaType:     mediaType,
						Digest:        srcDigest,
						ContentDigest: contentDigest,
						ListDigest:    listDigest,
						Config:        imageConfig,
						Layers:        layers,
						Manifest:      srcManifest,
					}, manifestErr); err != nil {
						return err
					}
				}
				return nil
			})
		}
	})
}
