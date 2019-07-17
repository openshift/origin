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
	"github.com/spf13/cobra"
	"k8s.io/klog"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/client"
	digest "github.com/opencontainers/go-digest"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	"github.com/openshift/api/image/docker10"
	"github.com/openshift/library-go/pkg/image/dockerv1client"
	imagereference "github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/library-go/pkg/image/registryclient"
	imagemanifest "github.com/openshift/oc/pkg/cli/image/manifest"
	"github.com/openshift/oc/pkg/cli/image/workqueue"
	"github.com/openshift/oc/pkg/helpers/image/dockerlayer"
	"github.com/openshift/oc/pkg/helpers/image/dockerlayer/add"
)

var (
	desc = templates.LongDesc(`
		Add layers to container images

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

	ConfigurationCallback func(dgst, contentDigest digest.Digest, config *dockerv1client.DockerImageConfig) error
	// ToDigest is set after a new image is uploaded
	ToDigest digest.Digest

	DropHistory bool
	CreatedAt   string

	SecurityOptions imagemanifest.SecurityOptions
	FilterOptions   imagemanifest.FilterOptions
	ParallelOptions imagemanifest.ParallelOptions

	DryRun bool
	Force  bool

	genericclioptions.IOStreams
}

func NewAppendImageOptions(streams genericclioptions.IOStreams) *AppendImageOptions {
	return &AppendImageOptions{
		IOStreams:       streams,
		ParallelOptions: imagemanifest.ParallelOptions{MaxPerRegistry: 4},
	}
}

// New creates a new command
func NewCmdAppendImage(name string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewAppendImageOptions(streams)

	cmd := &cobra.Command{
		Use:     "append",
		Short:   "Add layers to images and push them to a registry",
		Long:    desc,
		Example: fmt.Sprintf(example, name+" append"),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(c, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}

	flag := cmd.Flags()
	o.SecurityOptions.Bind(flag)
	o.FilterOptions.Bind(flag)
	o.ParallelOptions.Bind(flag)

	flag.BoolVar(&o.DryRun, "dry-run", o.DryRun, "Print the actions that would be taken and exit without writing to the destination.")

	flag.StringVar(&o.From, "from", o.From, "The image to use as a base. If empty, a new scratch image is created.")
	flag.StringVar(&o.To, "to", o.To, "The Docker repository tag to upload the appended image to.")

	flag.StringVar(&o.ConfigPatch, "image", o.ConfigPatch, "A JSON patch that will be used with the output image data.")
	flag.StringVar(&o.MetaPatch, "meta", o.MetaPatch, "A JSON patch that will be used with image base metadata (advanced config).")
	flag.BoolVar(&o.DropHistory, "drop-history", o.DropHistory, "Fields on the image that relate to the history of how the image was created will be removed.")
	flag.StringVar(&o.CreatedAt, "created-at", o.CreatedAt, "The creation date for this image, in RFC3339 format or milliseconds from the Unix epoch.")

	flag.BoolVar(&o.Force, "force", o.Force, "If set, the command will attempt to upload all layers instead of skipping those that are already uploaded.")

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

func (o *AppendImageOptions) Validate() error {
	return o.FilterOptions.Validate()
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

	ctx := context.Background()
	fromContext, err := o.SecurityOptions.Context()
	if err != nil {
		return err
	}
	toContext := fromContext.Copy().WithActions("pull", "push")

	toRepo, err := toContext.Repository(ctx, to.DockerClientDefaults().RegistryURL(), to.RepositoryName(), o.SecurityOptions.Insecure)
	if err != nil {
		return err
	}
	toManifests, err := toRepo.Manifests(ctx)
	if err != nil {
		return err
	}

	var (
		base              *dockerv1client.DockerImageConfig
		baseDigest        digest.Digest
		baseContentDigest digest.Digest
		layers            []distribution.Descriptor
		fromRepo          distribution.Repository
	)
	if from != nil {
		repo, err := fromContext.Repository(ctx, from.DockerClientDefaults().RegistryURL(), from.RepositoryName(), o.SecurityOptions.Insecure)
		if err != nil {
			return err
		}
		fromRepo = repo

		srcManifest, manifestLocation, err := imagemanifest.FirstManifest(ctx, *from, repo, o.FilterOptions.Include)
		if err != nil {
			return fmt.Errorf("unable to read image %s: %v", from, err)
		}
		base, layers, err = imagemanifest.ManifestToImageConfig(ctx, srcManifest, repo.Blobs(ctx), manifestLocation)
		if err != nil {
			return fmt.Errorf("unable to parse image %s: %v", from, err)
		}

		contentDigest, err := registryclient.ContentDigestForManifest(srcManifest, manifestLocation.Manifest.Algorithm())
		if err != nil {
			return err
		}

		baseDigest = manifestLocation.Manifest
		baseContentDigest = contentDigest

	} else {
		base = add.NewEmptyConfig()
		layers = nil
		fromRepo = scratchRepo{}
	}

	if base.Config == nil {
		base.Config = &docker10.DockerConfig{}
	}

	if o.ConfigurationCallback != nil {
		if err := o.ConfigurationCallback(baseDigest, baseContentDigest, base); err != nil {
			return err
		}
	} else {
		if klog.V(4) {
			configJSON, _ := json.MarshalIndent(base, "", "  ")
			klog.Infof("input config:\n%s\nlayers: %#v", configJSON, layers)
		}

		base.Parent = ""

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
	}

	if klog.V(4) {
		configJSON, _ := json.MarshalIndent(base, "", "  ")
		klog.Infof("output config:\n%s", configJSON)
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

	// all v1 schema images must have a history that equals the number of non-zero blob
	// layers, but v2 images do not require it
	for i := len(base.History); i < len(layers); i++ {
		base.History = append(base.History, dockerv1client.DockerConfigHistory{
			Created: base.Created,
		})
	}

	if o.DryRun {
		toManifests = &dryRunManifestService{}
		toBlobs = &dryRunBlobStore{layers: layers}
	}

	// upload base layers in parallel
	stopCh := make(chan struct{})
	defer close(stopCh)
	q := workqueue.New(o.ParallelOptions.MaxPerRegistry, stopCh)
	err = q.Try(func(w workqueue.Try) {
		for i := range layers[:numLayers] {
			layer := &layers[i]
			index := i
			needLayerDigest := len(base.RootFS.DiffIDs[i]) == 0
			w.Try(func() error {
				fromBlobs := fromRepo.Blobs(ctx)

				// check whether the blob exists
				if !o.Force {
					if desc, err := toBlobs.Stat(ctx, layer.Digest); err == nil {
						// ensure the correct size makes it back to the manifest
						klog.V(4).Infof("Layer %s already exists in destination (%s)", layer.Digest, units.HumanSizeWithPrecision(float64(layer.Size), 3))
						if layer.Size == 0 {
							layer.Size = desc.Size
						}
						// we need to calculate the tar sum from the image, requiring us to pull it
						if needLayerDigest {
							klog.V(4).Infof("Need tar sum, streaming layer %s", layer.Digest)
							r, err := fromBlobs.Open(ctx, layer.Digest)
							if err != nil {
								return fmt.Errorf("unable to access the layer %s in order to calculate its content ID: %v", layer.Digest, err)
							}
							defer r.Close()
							layerDigest, _, _, _, err := add.DigestCopy(ioutil.Discard.(io.ReaderFrom), r)
							if err != nil {
								return fmt.Errorf("unable to calculate contentID for layer %s: %v", layer.Digest, err)
							}
							klog.V(4).Infof("Layer %s has tar sum %s", layer.Digest, layerDigest)
							base.RootFS.DiffIDs[index] = layerDigest.String()
						}
						// TODO: due to a bug in the registry, the empty layer is always returned as existing, but
						// an upload without it will fail - https://bugzilla.redhat.com/show_bug.cgi?id=1599028
						if layer.Digest != dockerlayer.GzippedEmptyLayerDigest {
							return nil
						}
					}
				}

				// copy the blob, calculating layer digest if needed
				var mountFrom reference.Named
				if from != nil && from.Registry == to.Registry {
					mountFrom = fromRepo.Named()
				}
				desc, layerDigest, err := copyBlob(ctx, fromBlobs, toBlobs, *layer, o.Out, needLayerDigest, mountFrom)
				if err != nil {
					return fmt.Errorf("uploading the source layer %s failed: %v", layer.Digest, err)
				}
				if needLayerDigest {
					base.RootFS.DiffIDs[index] = layerDigest.String()
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

	manifest, configJSON, err := add.UploadSchema2Config(ctx, toBlobs, base, layers)
	if err != nil {
		return fmt.Errorf("unable to upload the new image manifest: %v", err)
	}
	klog.V(4).Infof("Created config JSON:\n%s", configJSON)
	toDigest, err := imagemanifest.PutManifestInCompatibleSchema(ctx, manifest, to.Tag, toManifests, toRepo.Named(), fromRepo.Blobs(ctx), configJSON)
	if err != nil {
		return fmt.Errorf("unable to convert the image to a compatible schema version: %v", err)
	}
	o.ToDigest = toDigest
	if !o.DryRun {
		fmt.Fprintf(o.Out, "Pushed %s to %s\n", toDigest, to)
	}
	return nil
}

// copyBlob attempts to mirror a blob from one repo to another, mounting it if possible, and calculating the
// layerDigest if needLayerDigest is true (mounting is not possible if we need to calculate a layerDigest).
func copyBlob(ctx context.Context, fromBlobs, toBlobs distribution.BlobService, layer distribution.Descriptor, out io.Writer, needLayerDigest bool, mountFrom reference.Named) (distribution.Descriptor, digest.Digest, error) {
	// source
	r, err := fromBlobs.Open(ctx, layer.Digest)
	if err != nil {
		return distribution.Descriptor{}, "", fmt.Errorf("unable to access the source layer %s: %v", layer.Digest, err)
	}
	defer r.Close()

	// destination
	mountOptions := []distribution.BlobCreateOption{WithDescriptor(layer)}
	if mountFrom != nil && !needLayerDigest {
		source, err := reference.WithDigest(mountFrom, layer.Digest)
		if err != nil {
			return distribution.Descriptor{}, "", err
		}
		mountOptions = append(mountOptions, client.WithMountFrom(source))
	}
	bw, err := toBlobs.Create(ctx, mountOptions...)
	if err != nil {
		switch t := err.(type) {
		case distribution.ErrBlobMounted:
			// mount successful
			klog.V(5).Infof("Blob mounted %#v", layer)
			if t.From.Digest() != layer.Digest {
				return distribution.Descriptor{}, "", fmt.Errorf("unable to upload layer %s to destination repository: tried to mount source and got back a different digest %s", layer.Digest, t.From.Digest())
			}
			if t.Descriptor.Size > 0 {
				layer.Size = t.Descriptor.Size
			}
			return layer, "", nil
		default:
			return distribution.Descriptor{}, "", fmt.Errorf("unable to upload layer %s to destination repository: %v", layer.Digest, err)
		}
	}
	defer bw.Close()

	if layer.Size > 0 {
		fmt.Fprintf(out, "Uploading %s ...\n", units.HumanSize(float64(layer.Size)))
	} else {
		fmt.Fprintf(out, "Uploading ...\n")
	}

	// copy the blob, calculating the diffID if necessary
	var layerDigest digest.Digest
	if needLayerDigest {
		klog.V(4).Infof("Need tar sum, calculating while streaming %s", layer.Digest)
		calculatedDigest, _, _, _, err := add.DigestCopy(bw, r)
		if err != nil {
			return distribution.Descriptor{}, "", err
		}
		layerDigest = calculatedDigest
		klog.V(4).Infof("Layer %s has tar sum %s", layer.Digest, layerDigest)

	} else {
		if _, err := bw.ReadFrom(r); err != nil {
			return distribution.Descriptor{}, "", err
		}
	}

	desc, err := bw.Commit(ctx, layer)
	if err != nil {
		return distribution.Descriptor{}, "", err
	}
	return desc, layerDigest, nil
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

func appendFileAsLayer(ctx context.Context, name string, layers []distribution.Descriptor, config *dockerv1client.DockerImageConfig, dryRun bool, out io.Writer,
	blobs distribution.BlobService) ([]distribution.Descriptor, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return appendLayer(ctx, f, layers, config, dryRun, out, blobs)
}

func appendLayer(ctx context.Context, r io.Reader, layers []distribution.Descriptor, config *dockerv1client.DockerImageConfig, dryRun bool, out io.Writer, blobs distribution.BlobService) ([]distribution.Descriptor,
	error) {
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

// dryRunManifestService emulates a remote registry for dry run behavior
type dryRunManifestService struct{}

func (s *dryRunManifestService) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	panic("not implemented")
}

func (s *dryRunManifestService) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	panic("not implemented")
}

func (s *dryRunManifestService) Put(ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption) (digest.Digest, error) {
	klog.V(4).Infof("Manifest: %#v", manifest.References())
	return registryclient.ContentDigestForManifest(manifest, digest.SHA256)
}

func (s *dryRunManifestService) Delete(ctx context.Context, dgst digest.Digest) error {
	panic("not implemented")
}

// dryRunBlobStore emulates a remote registry for dry run behavior
type dryRunBlobStore struct {
	layers []distribution.Descriptor
}

func (s *dryRunBlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	for _, layer := range s.layers {
		if layer.Digest == dgst {
			return layer, nil
		}
	}
	return distribution.Descriptor{}, distribution.ErrBlobUnknown
}

func (s *dryRunBlobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	panic("not implemented")
}

func (s *dryRunBlobStore) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	panic("not implemented")
}

func (s *dryRunBlobStore) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	return distribution.Descriptor{
		MediaType: mediaType,
		Size:      int64(len(p)),
		Digest:    digest.SHA256.FromBytes(p),
	}, nil
}

func (s *dryRunBlobStore) Create(ctx context.Context, options ...distribution.BlobCreateOption) (distribution.BlobWriter, error) {
	panic("not implemented")
}

func (s *dryRunBlobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	panic("not implemented")
}

func (s *dryRunBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, r *http.Request, dgst digest.Digest) error {
	panic("not implemented")
}

func (s *dryRunBlobStore) Delete(ctx context.Context, dgst digest.Digest) error {
	panic("not implemented")
}
