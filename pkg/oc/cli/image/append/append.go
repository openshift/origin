package append

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	units "github.com/docker/go-units"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/client"
	digest "github.com/opencontainers/go-digest"

	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/image/apis/image/docker10"
	imagereference "github.com/openshift/origin/pkg/image/apis/image/reference"
	"github.com/openshift/origin/pkg/image/dockerlayer"
	"github.com/openshift/origin/pkg/image/dockerlayer/add"
	"github.com/openshift/origin/pkg/image/registryclient"
	"github.com/openshift/origin/pkg/image/registryclient/dockercredentials"
	imagemanifest "github.com/openshift/origin/pkg/oc/cli/image/manifest"
	"github.com/openshift/origin/pkg/oc/cli/image/workqueue"
)

var (
	desc = templates.LongDesc(`
		Add layers to Docker images

		Modifies an existing image by adding layers or changing configuration and then pushes that
		image to a remote registry. Any inherited layers are streamed from registry to registry 
		without being stored locally. The default docker credentials are used for authenticating 
		to the registries.

		Layers may be provided as arguments to the command and must each be a gzipped tar archive
		representing a filesystem overlay to the inherited images. The archive may contain a "whiteout"
		file (the prefix '.wh.' and the filename) which will hide files in the lower layers. All
		supported filesystem attributes present in the archive will be used as is.

		Metadata about the image (the configuration passed to the container runtime) may be altered
		by passing a JSON string to the --image or --meta options. The --image flag changes what
		the container runtime sees, while the --meta option allows you to change the attributes of
		the image used by the runtime. Use --dry-run to see the result of your changes. You may
		add the --drop-history flag to remove information from the image about the system that 
		built the base image.

		Images in manifest list format will automatically select an image that matches the current
		operating system and architecture unless you use --filter-by-os to select a different image.
		This flag has no effect on regular images.
		`)

	example = templates.Examples(`
# Remove the entrypoint on the mysql:latest image
%[1]s --from mysql:latest --to myregistry.com/myimage:latest --image {"Entrypoint":null}

# Add a new layer to the image
%[1]s --from mysql:latest --to myregistry.com/myimage:latest layer.tar.gz
`)
)

type AppendImageOptions struct {
	From, To    string
	LayerFiles  []string
	LayerStream io.Reader

	ConfigPatch string
	MetaPatch   string

	DropHistory bool
	CreatedAt   string

	FilterOptions imagemanifest.FilterOptions

	MaxPerRegistry int

	DryRun   bool
	Insecure bool
	Force    bool

	genericclioptions.IOStreams
}

func NewAppendImageOptions(streams genericclioptions.IOStreams) *AppendImageOptions {
	return &AppendImageOptions{
		IOStreams:      streams,
		MaxPerRegistry: 3,
	}
}

// New creates a new command
func NewCmdAppendImage(name string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewAppendImageOptions(streams)

	cmd := &cobra.Command{
		Use:     "append",
		Short:   "Add layers to images and push them to a registry",
		Long:    desc,
		Example: fmt.Sprintf(example, name),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(c, args))
			kcmdutil.CheckErr(o.Run())
		},
	}

	flag := cmd.Flags()
	o.FilterOptions.Bind(flag)

	flag.BoolVar(&o.DryRun, "dry-run", o.DryRun, "Print the actions that would be taken and exit without writing to the destination.")
	flag.BoolVar(&o.Insecure, "insecure", o.Insecure, "Allow push and pull operations to registries to be made over HTTP")

	flag.StringVar(&o.From, "from", o.From, "The image to use as a base. If empty, a new scratch image is created.")
	flag.StringVar(&o.To, "to", o.To, "The Docker repository tag to upload the appended image to.")

	flag.StringVar(&o.ConfigPatch, "image", o.ConfigPatch, "A JSON patch that will be used with the output image data.")
	flag.StringVar(&o.MetaPatch, "meta", o.MetaPatch, "A JSON patch that will be used with image base metadata (advanced config).")
	flag.BoolVar(&o.DropHistory, "drop-history", o.DropHistory, "Fields on the image that relate to the history of how the image was created will be removed.")
	flag.StringVar(&o.CreatedAt, "created-at", o.CreatedAt, "The creation date for this image, in RFC3339 format or milliseconds from the Unix epoch.")

	flag.BoolVar(&o.Force, "force", o.Force, "If set, the command will attempt to upload all layers instead of skipping those that are already uploaded.")
	flag.IntVar(&o.MaxPerRegistry, "max-per-registry", o.MaxPerRegistry, "Number of concurrent requests allowed per registry.")

	return cmd
}

func (o *AppendImageOptions) Complete(cmd *cobra.Command, args []string) error {
	if err := o.FilterOptions.Complete(cmd.Flags()); err != nil {
		return err
	}

	for _, arg := range args {
		if arg == "-" {
			if o.LayerStream != nil {
				return fmt.Errorf("you may only specify '-' as an argument one time")
			}
			o.LayerStream = o.In
			continue
		}
		fi, err := os.Stat(arg)
		if err != nil {
			return fmt.Errorf("invalid argument: %s", err)
		}
		if fi.IsDir() {
			return fmt.Errorf("invalid argument: %s is a directory", arg)
		}
		o.LayerFiles = append(o.LayerFiles, arg)
	}

	return nil
}

func (o *AppendImageOptions) Run() error {
	var createdAt *time.Time
	if len(o.CreatedAt) > 0 {
		if d, err := strconv.ParseInt(o.CreatedAt, 10, 64); err == nil {
			t := time.Unix(d/1000, (d%1000)*1000000).UTC()
			createdAt = &t
		} else {
			t, err := time.Parse(time.RFC3339, o.CreatedAt)
			if err != nil {
				return fmt.Errorf("--created-at must be a relative time (2m, -5h) or an RFC3339 formatted date")
			}
			createdAt = &t
		}
	}

	var from *imagereference.DockerImageReference
	if len(o.From) > 0 {
		src, err := imagereference.Parse(o.From)
		if err != nil {
			return err
		}
		if len(src.Tag) == 0 && len(src.ID) == 0 {
			return fmt.Errorf("--from must point to an image ID or image tag")
		}
		from = &src
	}
	to, err := imagereference.Parse(o.To)
	if err != nil {
		return err
	}
	if len(to.ID) > 0 {
		return fmt.Errorf("--to may not point to an image by ID")
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
	ctx := context.Background()
	fromContext := registryclient.NewContext(rt, insecureRT).WithCredentials(creds)
	toContext := registryclient.NewContext(rt, insecureRT).WithActions("push").WithCredentials(creds)

	toRepo, err := toContext.Repository(ctx, to.DockerClientDefaults().RegistryURL(), to.RepositoryName(), o.Insecure)
	if err != nil {
		return err
	}
	toManifests, err := toRepo.Manifests(ctx)
	if err != nil {
		return err
	}

	var (
		base     *docker10.DockerImageConfig
		layers   []distribution.Descriptor
		fromRepo distribution.Repository
	)
	if from != nil {
		repo, err := fromContext.Repository(ctx, from.DockerClientDefaults().RegistryURL(), from.RepositoryName(), o.Insecure)
		if err != nil {
			return err
		}
		fromRepo = repo

		srcManifest, _, location, err := imagemanifest.FirstManifest(ctx, *from, repo, o.FilterOptions.Include)
		if err != nil {
			return fmt.Errorf("unable to read image %s: %v", from, err)
		}
		base, layers, err = imagemanifest.ManifestToImageConfig(ctx, srcManifest, repo.Blobs(ctx), location)
		if err != nil {
			return fmt.Errorf("unable to parse image %s: %v", from, err)
		}

	} else {
		base = add.NewEmptyConfig()
		layers = nil
		fromRepo = scratchRepo{}
	}

	if base.Config == nil {
		base.Config = &docker10.DockerConfig{}
	}

	if glog.V(4) {
		configJSON, _ := json.MarshalIndent(base, "", "  ")
		glog.Infof("input config:\n%s\nlayers: %#v", configJSON, layers)
	}

	if createdAt == nil {
		t := time.Now()
		createdAt = &t
	}
	base.Created = *createdAt
	if o.DropHistory {
		base.ContainerConfig = docker10.DockerConfig{}
		base.History = nil
		base.Container = ""
		base.DockerVersion = ""
		base.Config.Image = ""
	}

	if len(o.ConfigPatch) > 0 {
		if err := json.Unmarshal([]byte(o.ConfigPatch), base.Config); err != nil {
			return fmt.Errorf("unable to patch image from --image: %v", err)
		}
	}
	if len(o.MetaPatch) > 0 {
		if err := json.Unmarshal([]byte(o.MetaPatch), base); err != nil {
			return fmt.Errorf("unable to patch image from --meta: %v", err)
		}
	}

	if glog.V(4) {
		configJSON, _ := json.MarshalIndent(base, "", "  ")
		glog.Infof("output config:\n%s", configJSON)
	}

	numLayers := len(layers)
	toBlobs := toRepo.Blobs(ctx)

	for _, arg := range o.LayerFiles {
		layers, err = appendFileAsLayer(ctx, arg, layers, base, o.DryRun, o.Out, toBlobs)
		if err != nil {
			return err
		}
	}
	if o.LayerStream != nil {
		layers, err = appendLayer(ctx, o.LayerStream, layers, base, o.DryRun, o.Out, toBlobs)
		if err != nil {
			return err
		}
	}
	if len(layers) == 0 {
		layers, err = appendLayer(ctx, bytes.NewBuffer(dockerlayer.GzippedEmptyLayer), layers, base, o.DryRun, o.Out, toBlobs)
		if err != nil {
			return err
		}
	}

	if o.DryRun {
		configJSON, _ := json.MarshalIndent(base, "", "  ")
		fmt.Fprintf(o.Out, "%s", configJSON)
		return nil
	}

	// upload base layers in parallel
	stopCh := make(chan struct{})
	defer close(stopCh)
	q := workqueue.New(o.MaxPerRegistry, stopCh)
	err = q.Try(func(w workqueue.Try) {
		for i := range layers[:numLayers] {
			layer := &layers[i]
			index := i
			missingDiffID := len(base.RootFS.DiffIDs[i]) == 0
			w.Try(func() error {
				fromBlobs := fromRepo.Blobs(ctx)

				// check whether the blob exists
				if !o.Force {
					if desc, err := toBlobs.Stat(ctx, layer.Digest); err == nil {
						// ensure the correct size makes it back to the manifest
						glog.V(4).Infof("Layer %s already exists in destination (%s)", layer.Digest, units.HumanSizeWithPrecision(float64(layer.Size), 3))
						if layer.Size == 0 {
							layer.Size = desc.Size
						}
						// we need to calculate the tar sum from the image, requiring us to pull it
						if missingDiffID {
							glog.V(4).Infof("Need tar sum, streaming layer %s", layer.Digest)
							r, err := fromBlobs.Open(ctx, layer.Digest)
							if err != nil {
								return fmt.Errorf("unable to access the layer %s in order to calculate its content ID: %v", layer.Digest, err)
							}
							defer r.Close()
							layerDigest, _, _, _, err := add.DigestCopy(ioutil.Discard.(io.ReaderFrom), r)
							if err != nil {
								return fmt.Errorf("unable to calculate contentID for layer %s: %v", layer.Digest, err)
							}
							glog.V(4).Infof("Layer %s has tar sum %s", layer.Digest, layerDigest)
							base.RootFS.DiffIDs[index] = layerDigest.String()
						}
						// TODO: due to a bug in the registry, the empty layer is always returned as existing, but
						// an upload without it will fail - https://bugzilla.redhat.com/show_bug.cgi?id=1599028
						if layer.Digest != dockerlayer.GzippedEmptyLayerDigest {
							return nil
						}
					}
				}

				// source
				r, err := fromBlobs.Open(ctx, layer.Digest)
				if err != nil {
					return fmt.Errorf("unable to access the source layer %s: %v", layer.Digest, err)
				}
				defer r.Close()

				// destination
				mountOptions := []distribution.BlobCreateOption{WithDescriptor(*layer)}
				if from != nil && from.Registry == to.Registry {
					source, err := reference.WithDigest(fromRepo.Named(), layer.Digest)
					if err != nil {
						return err
					}
					mountOptions = append(mountOptions, client.WithMountFrom(source))
				}
				bw, err := toBlobs.Create(ctx, mountOptions...)
				if err != nil {
					return fmt.Errorf("unable to upload layer %s to destination repository: %v", layer.Digest, err)
				}
				defer bw.Close()

				// copy the blob, calculating the diffID if necessary
				if layer.Size > 0 {
					fmt.Fprintf(o.Out, "Uploading %s ...\n", units.HumanSize(float64(layer.Size)))
				} else {
					fmt.Fprintf(o.Out, "Uploading ...\n")
				}
				if missingDiffID {
					glog.V(4).Infof("Need tar sum, calculating while streaming %s", layer.Digest)
					layerDigest, _, _, _, err := add.DigestCopy(bw, r)
					if err != nil {
						return err
					}
					glog.V(4).Infof("Layer %s has tar sum %s", layer.Digest, layerDigest)
					base.RootFS.DiffIDs[index] = layerDigest.String()
				} else {
					if _, err := bw.ReadFrom(r); err != nil {
						return fmt.Errorf("unable to copy the source layer %s to the destination image: %v", layer.Digest, err)
					}
				}
				desc, err := bw.Commit(ctx, *layer)
				if err != nil {
					return fmt.Errorf("uploading the source layer %s failed: %v", layer.Digest, err)
				}

				// check output
				if desc.Digest != layer.Digest {
					return fmt.Errorf("when uploading blob %s, got a different returned digest %s", desc.Digest, layer.Digest)
				}
				// ensure the correct size makes it back to the manifest
				if layer.Size == 0 {
					layer.Size = desc.Size
				}
				return nil
			})
		}
	})
	if err != nil {
		return err
	}

	manifest, err := add.UploadSchema2Config(ctx, toBlobs, base, layers)
	if err != nil {
		return fmt.Errorf("unable to upload the new image manifest: %v", err)
	}
	toDigest, err := imagemanifest.PutManifestInCompatibleSchema(ctx, manifest, to.Tag, toManifests, fromRepo.Blobs(ctx), toRepo.Named())
	if err != nil {
		return fmt.Errorf("unable to convert the image to a compatible schema version: %v", err)
	}
	fmt.Fprintf(o.Out, "Pushed image %s to %s\n", toDigest, to)
	return nil
}

type optionFunc func(interface{}) error

func (f optionFunc) Apply(v interface{}) error {
	return f(v)
}

// WithDescriptor returns a BlobCreateOption which provides the expected blob metadata.
func WithDescriptor(desc distribution.Descriptor) distribution.BlobCreateOption {
	return optionFunc(func(v interface{}) error {
		opts, ok := v.(*distribution.CreateOptions)
		if !ok {
			return fmt.Errorf("unexpected options type: %T", v)
		}
		if opts.Mount.Stat == nil {
			opts.Mount.Stat = &desc
		}
		return nil
	})
}

func appendFileAsLayer(ctx context.Context, name string, layers []distribution.Descriptor, config *docker10.DockerImageConfig, dryRun bool, out io.Writer, blobs distribution.BlobService) ([]distribution.Descriptor, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return appendLayer(ctx, f, layers, config, dryRun, out, blobs)
}

func appendLayer(ctx context.Context, r io.Reader, layers []distribution.Descriptor, config *docker10.DockerImageConfig, dryRun bool, out io.Writer, blobs distribution.BlobService) ([]distribution.Descriptor, error) {
	var readerFrom io.ReaderFrom = ioutil.Discard.(io.ReaderFrom)
	var done = func(distribution.Descriptor) error { return nil }
	if !dryRun {
		fmt.Fprint(out, "Uploading ... ")
		start := time.Now()
		bw, err := blobs.Create(ctx)
		if err != nil {
			fmt.Fprintln(out, "failed")
			return nil, err
		}
		readerFrom = bw
		defer bw.Close()
		done = func(desc distribution.Descriptor) error {
			_, err := bw.Commit(ctx, desc)
			if err != nil {
				fmt.Fprintln(out, "failed")
				return err
			}
			fmt.Fprintf(out, "%s/s\n", units.HumanSize(float64(desc.Size)/float64(time.Now().Sub(start))*float64(time.Second)))
			return nil
		}
	}
	layerDigest, blobDigest, modTime, n, err := add.DigestCopy(readerFrom, r)
	if err != nil {
		return nil, err
	}
	desc := distribution.Descriptor{
		Digest:    blobDigest,
		Size:      n,
		MediaType: schema2.MediaTypeLayer,
	}
	layers = append(layers, desc)
	add.AddLayerToConfig(config, desc, layerDigest.String())
	if modTime != nil && !modTime.IsZero() {
		config.Created = *modTime
	}
	return layers, done(desc)
}

func calculateLayerDigest(blobs distribution.BlobService, dgst digest.Digest, readerFrom io.ReaderFrom, r io.Reader) (digest.Digest, error) {
	if readerFrom == nil {
		readerFrom = ioutil.Discard.(io.ReaderFrom)
	}
	layerDigest, _, _, _, err := add.DigestCopy(readerFrom, r)
	return layerDigest, err
}

// scratchRepo can serve the scratch image blob.
type scratchRepo struct{}

var _ distribution.Repository = scratchRepo{}

func (_ scratchRepo) Named() reference.Named { panic("not implemented") }
func (_ scratchRepo) Tags(ctx context.Context) distribution.TagService {
	panic("not implemented")
}
func (_ scratchRepo) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {
	panic("not implemented")
}

func (r scratchRepo) Blobs(ctx context.Context) distribution.BlobStore { return r }

func (_ scratchRepo) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	if dgst != dockerlayer.GzippedEmptyLayerDigest {
		return distribution.Descriptor{}, distribution.ErrBlobUnknown
	}
	return distribution.Descriptor{
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Digest:    digest.Digest(dockerlayer.GzippedEmptyLayerDigest),
		Size:      int64(len(dockerlayer.GzippedEmptyLayer)),
	}, nil
}

func (_ scratchRepo) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	if dgst != dockerlayer.GzippedEmptyLayerDigest {
		return nil, distribution.ErrBlobUnknown
	}
	return dockerlayer.GzippedEmptyLayer, nil
}

type nopCloseBuffer struct {
	*bytes.Buffer
}

func (_ nopCloseBuffer) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

func (_ nopCloseBuffer) Close() error {
	return nil
}

func (_ scratchRepo) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	if dgst != dockerlayer.GzippedEmptyLayerDigest {
		return nil, distribution.ErrBlobUnknown
	}
	return nopCloseBuffer{bytes.NewBuffer(dockerlayer.GzippedEmptyLayer)}, nil
}

func (_ scratchRepo) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	panic("not implemented")
}

func (_ scratchRepo) Create(ctx context.Context, options ...distribution.BlobCreateOption) (distribution.BlobWriter, error) {
	panic("not implemented")
}

func (_ scratchRepo) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	panic("not implemented")
}

func (_ scratchRepo) ServeBlob(ctx context.Context, w http.ResponseWriter, r *http.Request, dgst digest.Digest) error {
	panic("not implemented")
}

func (_ scratchRepo) Delete(ctx context.Context, dgst digest.Digest) error {
	panic("not implemented")
}
