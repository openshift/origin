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

	"github.com/openshift/origin/pkg/oc/cli/image/workqueue"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	units "github.com/docker/go-units"
	digest "github.com/opencontainers/go-digest"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/image/apis/image/docker10"
	imagereference "github.com/openshift/origin/pkg/image/apis/image/reference"
	"github.com/openshift/origin/pkg/image/registryclient"
	"github.com/openshift/origin/pkg/image/registryclient/dockercredentials"
	imagemanifest "github.com/openshift/origin/pkg/oc/cli/image/manifest"
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
	flags.StringVarP(&o.RegistryConfig, "registry-config", "a", o.RegistryConfig, "Path to your registry credentials (defaults to ~/.docker/config.json)")
	flags.StringVarP(&o.Output, "output", "o", o.Output, "Print the image in an alternative format: json")
	flags.BoolVar(&o.Insecure, "insecure", o.Insecure, "Allow push and pull operations to registries to be made over HTTP")
	return cmd
}

type InfoOptions struct {
	genericclioptions.IOStreams

	RegistryConfig string

	FilterOptions imagemanifest.FilterOptions

	Images []string

	Output   string
	Insecure bool
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

	for _, location := range o.Images {
		src, err := imagereference.Parse(location)
		if err != nil {
			return err
		}
		if len(src.Tag) == 0 && len(src.ID) == 0 {
			return fmt.Errorf("--from must point to an image ID or image tag")
		}

		rt, err := rest.TransportFor(&rest.Config{})
		if err != nil {
			return err
		}
		insecureRT, err := rest.TransportFor(&rest.Config{TLSClientConfig: rest.TLSClientConfig{Insecure: true}})
		if err != nil {
			return err
		}
		creds := dockercredentials.NewLocal()
		if len(o.RegistryConfig) > 0 {
			creds, err = dockercredentials.NewFromFile(o.RegistryConfig)
			if err != nil {
				return fmt.Errorf("unable to load --registry-config: %v", err)
			}
		}
		ctx := context.Background()
		context := registryclient.NewContext(rt, insecureRT).WithCredentials(creds)

		repo, err := context.Repository(ctx, src.DockerClientDefaults().RegistryURL(), src.RepositoryName(), o.Insecure)
		if err != nil {
			return err
		}

		srcManifest, srcDigest, _, err := imagemanifest.FirstManifest(ctx, src, repo, o.FilterOptions.IncludeAll)
		if err != nil {
			return fmt.Errorf("unable to read image %s: %v", location, err)
		}

		switch t := srcManifest.(type) {
		case *manifestlist.DeserializedManifestList:
			buf := &bytes.Buffer{}
			w := tabwriter.NewWriter(buf, 0, 0, 1, ' ', 0)
			fmt.Fprintf(w, "  OS\tDIGEST\n")
			for _, manifest := range t.Manifests {
				fmt.Fprintf(w, "  %s\t%s\n", imagemanifest.PlatformSpecString(manifest.Platform), manifest.Digest)
			}
			w.Flush()
			return fmt.Errorf("the image is a manifest list and contains multiple images - use --filter-by-os to select from:\n\n%s\n", buf.String())
		}

		imageConfig, layers, err := imagemanifest.ManifestToImageConfig(ctx, srcManifest, repo.Blobs(ctx), location)
		if err != nil {
			return fmt.Errorf("unable to parse image %s: %v", location, err)
		}

		mediaType, _, _ := srcManifest.Payload()

		image := &Image{
			Name:      location,
			Ref:       src,
			Config:    imageConfig,
			Digest:    srcDigest,
			MediaType: mediaType,
			Layers:    layers,
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
			return err
		}
	}
	return nil
}

type Image struct {
	Name      string                              `json:"name"`
	Ref       imagereference.DockerImageReference `json:"-"`
	Digest    digest.Digest                       `json:"digest"`
	MediaType string                              `json:"mediaType"`
	Layers    []distribution.Descriptor           `json:"layers"`
	Config    *docker10.DockerImageConfig         `json:"config"`
}

func describeImage(out io.Writer, image *Image) error {
	w := tabwriter.NewWriter(out, 0, 4, 1, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "Name:\t%s\n", image.Name)
	if len(image.Ref.ID) == 0 {
		fmt.Fprintf(w, "Digest:\t%s\n", image.Digest)
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
	case 1:
		fmt.Fprintf(w, "Image Size:\t%s\n", units.HumanSize(float64(image.Layers[0].Size)))
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
	return nil
}

func writeTabSection(out io.Writer, fn func(w io.Writer)) {
	w := tabwriter.NewWriter(out, 0, 4, 1, ' ', 0)
	fn(w)
	w.Flush()
}

type ImageRetriever struct {
	Image          map[string]imagereference.DockerImageReference
	Insecure       bool
	RegistryConfig string
	MaxPerRegistry int
	// ImageMetadataCallback is invoked once per image retrieved, and may be called in parallel if
	// MaxPerRegistry is set higher than 1. If err is passed image is nil. If an error is returned
	// execution will stop.
	ImageMetadataCallback func(from string, image *Image, err error) error
}

func (o *ImageRetriever) Run() error {
	rt, err := rest.TransportFor(&rest.Config{})
	if err != nil {
		return err
	}
	insecureRT, err := rest.TransportFor(&rest.Config{TLSClientConfig: rest.TLSClientConfig{Insecure: true}})
	if err != nil {
		return err
	}
	creds := dockercredentials.NewLocal()
	if len(o.RegistryConfig) > 0 {
		creds, err = dockercredentials.NewFromFile(o.RegistryConfig)
		if err != nil {
			return fmt.Errorf("unable to load --registry-config: %v", err)
		}
	}
	ctx := context.Background()
	fromContext := registryclient.NewContext(rt, insecureRT).WithCredentials(creds)

	stopCh := make(chan struct{})
	defer close(stopCh)
	q := workqueue.New(o.MaxPerRegistry, stopCh)
	return q.Try(func(q workqueue.Try) {
		for key := range o.Image {
			name := key
			from := o.Image[key]
			q.Try(func() error {
				repo, err := fromContext.Repository(ctx, from.DockerClientDefaults().RegistryURL(), from.RepositoryName(), o.Insecure)
				if err != nil {
					return fmt.Errorf("unable to connect to image repository %s: %v", from.Exact(), err)
				}

				allManifests, _, _, err := imagemanifest.AllManifests(ctx, from, repo)
				if err != nil {
					if imagemanifest.IsImageForbidden(err) {
						var msg string
						if len(o.Image) == 1 {
							msg = "image does not exist or you don't have permission to access the repository"
						} else {
							msg = fmt.Sprintf("image %q does not exist or you don't have permission to access the repository", from)
						}
						return imagemanifest.NewImageForbidden(msg, err)
					}
					if imagemanifest.IsImageNotFound(err) {
						var msg string
						if len(o.Image) == 1 {
							msg = "image does not exist"
						} else {
							msg = fmt.Sprintf("image %q does not exist", from)
						}
						return imagemanifest.NewImageNotFound(msg, err)
					}
					return fmt.Errorf("unable to read image %s: %v", from, err)
				}

				for srcDigest, srcManifest := range allManifests {
					imageConfig, layers, err := imagemanifest.ManifestToImageConfig(ctx, srcManifest, repo.Blobs(ctx), from.Exact())
					if o.ImageMetadataCallback != nil {
						mediaType, _, _ := srcManifest.Payload()

						if err := o.ImageMetadataCallback(name, &Image{
							Name:      from.Exact(),
							MediaType: mediaType,
							Digest:    srcDigest,
							Config:    imageConfig,
							Layers:    layers,
						}, err); err != nil {
							return err
						}
					} else {
						if err != nil {
							return err
						}
					}
				}
				return nil
			})
		}
	})
}
