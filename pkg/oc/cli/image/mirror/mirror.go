package mirror

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/client"

	units "github.com/docker/go-units"
	"github.com/golang/glog"
	godigest "github.com/opencontainers/go-digest"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"

	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	imagereference "github.com/openshift/origin/pkg/image/apis/image/reference"
	"github.com/openshift/origin/pkg/image/registryclient"
	"github.com/openshift/origin/pkg/image/registryclient/dockercredentials"
	imagemanifest "github.com/openshift/origin/pkg/oc/cli/image/manifest"
	"github.com/openshift/origin/pkg/oc/cli/image/workqueue"
)

var (
	mirrorDesc = templates.LongDesc(`
		Mirror images from one image repository to another.

		Accepts a list of arguments defining source images that should be pushed to the provided
		destination image tag. The images are streamed from registry to registry without being stored
		locally. The default docker credentials are used for authenticating to the registries.

		When using S3 mirroring the region and bucket must be the first two segments after the host.
		Mirroring will create the necessary metadata so that images can be pulled via tag or digest,
		but listing manifests and tags will not be possible. You may also specify one or more
		--s3-source-bucket parameters (as <bucket>/<path>) to designate buckets to look in to find
		blobs (instead of uploading). The source bucket also supports the suffix "/[store]", which
		will transform blob identifiers into the form the Docker registry uses on disk, allowing
		you to mirror directly from an existing S3-backed Docker registry. Credentials for S3
		may be stored in your docker credential file and looked up by host.

		Images in manifest list format will be copied as-is unless you use --filter-by-os to restrict
		the allowed images to copy in a manifest list. This flag has no effect on regular images.
		`)

	mirrorExample = templates.Examples(`
# Copy image to another tag
%[1]s myregistry.com/myimage:latest myregistry.com/myimage:stable

# Copy image to another registry
%[1]s myregistry.com/myimage:latest docker.io/myrepository/myimage:stable

# Copy image to S3 (pull from <bucket>.s3.amazonaws.com/image:latest)
%[1]s myregistry.com/myimage:latest s3://s3.amazonaws.com/<region>/<bucket>/image:latest

# Copy image to S3 without setting a tag (pull via @<digest>)
%[1]s myregistry.com/myimage:latest s3://s3.amazonaws.com/<region>/<bucket>/image

# Copy image to multiple locations
%[1]s myregistry.com/myimage:latest docker.io/myrepository/myimage:stable \
    docker.io/myrepository/myimage:dev

# Copy multiple images
%[1]s myregistry.com/myimage:latest=myregistry.com/other:test \
    myregistry.com/myimage:new=myregistry.com/other:target
`)
)

type MirrorImageOptions struct {
	Mappings []Mapping

	FilterOptions imagemanifest.FilterOptions

	DryRun             bool
	Insecure           bool
	SkipMount          bool
	SkipMultipleScopes bool
	Force              bool

	MaxRegistry    int
	MaxPerRegistry int

	AttemptS3BucketCopy []string

	Filenames []string

	genericclioptions.IOStreams
}

func NewMirrorImageOptions(streams genericclioptions.IOStreams) *MirrorImageOptions {
	return &MirrorImageOptions{
		IOStreams: streams,
	}
}

// NewCommandMirrorImage copies images from one location to another.
func NewCmdMirrorImage(name string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewMirrorImageOptions(streams)

	cmd := &cobra.Command{
		Use:     "mirror SRC DST [DST ...]",
		Short:   "Mirror images from one repository to another",
		Long:    mirrorDesc,
		Example: fmt.Sprintf(mirrorExample, name+" mirror"),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(c, args))
			kcmdutil.CheckErr(o.Run())
		},
	}

	flag := cmd.Flags()
	o.FilterOptions.Bind(flag)

	flag.BoolVar(&o.DryRun, "dry-run", o.DryRun, "Print the actions that would be taken and exit without writing to the destinations.")
	flag.BoolVar(&o.Insecure, "insecure", o.Insecure, "Allow push and pull operations to registries to be made over HTTP")
	flag.BoolVar(&o.SkipMount, "skip-mount", o.SkipMount, "Always push layers instead of cross-mounting them")
	flag.BoolVar(&o.SkipMultipleScopes, "skip-multiple-scopes", o.SkipMultipleScopes, "Some registries do not support multiple scopes passed to the registry login.")
	flag.BoolVar(&o.Force, "force", o.Force, "Attempt to write all layers and manifests even if they exist in the remote repository.")
	flag.IntVar(&o.MaxRegistry, "max-registry", 4, "Number of concurrent registries to connect to at any one time.")
	flag.IntVar(&o.MaxPerRegistry, "max-per-registry", 6, "Number of concurrent requests allowed per registry.")
	flag.StringSliceVar(&o.AttemptS3BucketCopy, "s3-source-bucket", o.AttemptS3BucketCopy, "A list of bucket/path locations on S3 that may contain already uploaded blobs. Add [store] to the end to use the Docker registry path convention.")
	flag.StringSliceVarP(&o.Filenames, "filename", "f", o.Filenames, "One or more files to read SRC=DST or SRC DST [DST ...] mappings from.")

	return cmd
}

func (o *MirrorImageOptions) Complete(cmd *cobra.Command, args []string) error {
	if err := o.FilterOptions.Complete(cmd.Flags()); err != nil {
		return err
	}

	overlap := make(map[string]string)

	var err error
	o.Mappings, err = parseArgs(args, overlap)
	if err != nil {
		return err
	}
	for _, filename := range o.Filenames {
		mappings, err := parseFile(filename, overlap)
		if err != nil {
			return err
		}
		o.Mappings = append(o.Mappings, mappings...)
	}

	if len(o.Mappings) == 0 {
		return fmt.Errorf("you must specify at least one source image to pull and the destination to push to as SRC=DST or SRC DST [DST2 DST3 ...]")
	}

	for _, mapping := range o.Mappings {
		if mapping.Source.Equal(mapping.Destination) {
			return fmt.Errorf("SRC and DST may not be the same")
		}
	}

	return nil
}

func (o *MirrorImageOptions) Repository(ctx context.Context, context *registryclient.Context, t DestinationType, ref imagereference.DockerImageReference) (distribution.Repository, error) {
	switch t {
	case DestinationRegistry:
		return context.Repository(ctx, ref.DockerClientDefaults().RegistryURL(), ref.RepositoryName(), o.Insecure)
	case DestinationS3:
		driver := &s3Driver{
			Creds:    context.Credentials,
			CopyFrom: o.AttemptS3BucketCopy,
		}
		url := ref.DockerClientDefaults().RegistryURL()
		return driver.Repository(ctx, url, ref.RepositoryName(), o.Insecure)
	default:
		return nil, fmt.Errorf("unrecognized destination type %s", t)
	}
}

func (o *MirrorImageOptions) Run() error {
	start := time.Now()
	p, err := o.plan()
	if err != nil {
		return err
	}
	p.Print(o.ErrOut)
	fmt.Fprintln(o.ErrOut)

	if errs := p.Errors(); len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintf(o.ErrOut, "error: %v\n", err)
		}
		return fmt.Errorf("an error occurred during planning")
	}

	work := Greedy(p)
	work.Print(o.ErrOut)
	fmt.Fprintln(o.ErrOut)

	fmt.Fprintf(o.ErrOut, "info: Planning completed in %s\n", time.Now().Sub(start).Round(10*time.Millisecond))

	if o.DryRun {
		fmt.Fprintf(o.ErrOut, "info: Dry run complete\n")
		return nil
	}

	stopCh := make(chan struct{})
	defer close(stopCh)
	q := workqueue.New(o.MaxRegistry, stopCh)
	registryWorkers := make(map[string]workqueue.Interface)
	for name := range p.RegistryNames() {
		registryWorkers[name] = workqueue.New(o.MaxPerRegistry, stopCh)
	}

	next := time.Now()
	defer func() {
		d := time.Now().Sub(next)
		fmt.Fprintf(o.ErrOut, "info: Mirroring completed in %s (%s/s)\n", d.Truncate(10*time.Millisecond), units.HumanSize(float64(work.stats.bytes)/d.Seconds()))
	}()

	ctx := apirequest.NewContext()
	for j := range work.phases {
		phase := &work.phases[j]
		q.Batch(func(w workqueue.Work) {
			for i := range phase.independent {
				unit := phase.independent[i]
				w.Parallel(func() {
					// upload blobs
					registryWorkers[unit.registry.name].Batch(func(w workqueue.Work) {
						for i := range unit.repository.blobs {
							op := unit.repository.blobs[i]
							for digestString := range op.blobs {
								digest := godigest.Digest(digestString)
								blob := op.parent.parent.parent.GetBlob(digest)
								w.Parallel(func() {
									if err := copyBlob(ctx, work, op, blob, o.Force, o.SkipMount, o.ErrOut); err != nil {
										fmt.Fprintf(o.ErrOut, "error: %v\n", err)
										phase.Failed()
										return
									}
									op.parent.parent.AssociateBlob(digest, unit.repository.name)
								})
							}
						}
					})
					if phase.IsFailed() {
						return
					}
					// upload manifests
					op := unit.repository.manifests
					if errs := copyManifests(ctx, op, o.Out); len(errs) > 0 {
						for _, err := range errs {
							fmt.Fprintf(o.ErrOut, "error: %v\n", err)
						}
						phase.Failed()
					}
				})
			}
		})
		if phase.IsFailed() {
			return fmt.Errorf("one or more errors occurred while uploading images")
		}
	}

	return nil
}

func (o *MirrorImageOptions) plan() (*plan, error) {
	rt, err := rest.TransportFor(&rest.Config{})
	if err != nil {
		return nil, err
	}
	insecureRT, err := rest.TransportFor(&rest.Config{TLSClientConfig: rest.TLSClientConfig{Insecure: true}})
	if err != nil {
		return nil, err
	}
	creds := dockercredentials.NewLocal()
	ctx := apirequest.NewContext()
	fromContext := registryclient.NewContext(rt, insecureRT).WithCredentials(creds)
	toContext := registryclient.NewContext(rt, insecureRT).WithActions("pull", "push").WithCredentials(creds)
	toContexts := make(map[string]*registryclient.Context)

	tree := buildTargetTree(o.Mappings)
	for registry, scopes := range calculateDockerRegistryScopes(tree) {
		glog.V(5).Infof("Using scopes for registry %s: %v", registry, scopes)
		if o.SkipMultipleScopes {
			toContexts[registry] = toContext.Copy()
		} else {
			toContexts[registry] = toContext.Copy().WithScopes(scopes...)
		}
	}

	stopCh := make(chan struct{})
	defer close(stopCh)
	q := workqueue.New(o.MaxRegistry, stopCh)
	registryWorkers := make(map[string]workqueue.Interface)
	for name := range tree {
		if _, ok := registryWorkers[name.registry]; !ok {
			registryWorkers[name.registry] = workqueue.New(o.MaxPerRegistry, stopCh)
		}
	}

	plan := newPlan()

	for name := range tree {
		src := tree[name]
		q.Queue(func(_ workqueue.Work) {
			srcRepo, err := fromContext.Repository(ctx, src.ref.DockerClientDefaults().RegistryURL(), src.ref.RepositoryName(), o.Insecure)
			if err != nil {
				plan.AddError(retrieverError{err: fmt.Errorf("unable to connect to %s: %v", src.ref, err), src: src.ref})
				return
			}
			manifests, err := srcRepo.Manifests(ctx)
			if err != nil {
				plan.AddError(retrieverError{src: src.ref, err: fmt.Errorf("unable to access source image %s manifests: %v", src.ref, err)})
				return
			}
			rq := registryWorkers[name.registry]
			rq.Batch(func(w workqueue.Work) {
				// convert source tags to digests
				for tag := range src.tags {
					srcTag, pushTargets := tag, src.tags[tag]
					w.Parallel(func() {
						desc, err := srcRepo.Tags(ctx).Get(ctx, srcTag)
						if err != nil {
							plan.AddError(retrieverError{src: src.ref, err: fmt.Errorf("unable to retrieve source image %s by tag %s: %v", src.ref, srcTag, err)})
							return
						}
						srcDigest := desc.Digest
						glog.V(3).Infof("Resolved source image %s:%s to %s\n", src.ref, srcTag, srcDigest)
						src.mergeIntoDigests(srcDigest, pushTargets)
					})
				}
			})

			canonicalFrom := srcRepo.Named()

			rq.Queue(func(w workqueue.Work) {
				for key := range src.digests {
					srcDigestString, pushTargets := key, src.digests[key]
					w.Parallel(func() {
						// load the manifest
						srcDigest := godigest.Digest(srcDigestString)
						srcManifest, err := manifests.Get(ctx, godigest.Digest(srcDigest), imagemanifest.PreferManifestList)
						if err != nil {
							plan.AddError(retrieverError{src: src.ref, err: fmt.Errorf("unable to retrieve source image %s manifest %s: %v", src.ref, srcDigest, err)})
							return
						}

						// filter or load manifest list as appropriate
						originalSrcDigest := srcDigest
						srcManifests, srcManifest, srcDigest, err := imagemanifest.ProcessManifestList(ctx, srcDigest, srcManifest, manifests, src.ref, o.FilterOptions.IncludeAll)
						if err != nil {
							plan.AddError(retrieverError{src: src.ref, err: err})
							return
						}
						if len(srcManifests) == 0 {
							fmt.Fprintf(o.ErrOut, "info: Filtered all images from %s, skipping\n", src.ref)
							return
						}

						var location string
						if srcDigest == originalSrcDigest {
							location = fmt.Sprintf("manifest %s", srcDigest)
						} else {
							location = fmt.Sprintf("manifest %s in manifest list %s", srcDigest, originalSrcDigest)
						}

						for _, dst := range pushTargets {
							toRepo, err := o.Repository(ctx, toContexts[dst.ref.Registry], dst.t, dst.ref)
							if err != nil {
								plan.AddError(retrieverError{src: src.ref, dst: dst.ref, err: fmt.Errorf("unable to connect to %s: %v", dst.ref, err)})
								continue
							}

							canonicalTo := toRepo.Named()

							repoPlan := plan.RegistryPlan(dst.ref.Registry).RepositoryPlan(canonicalTo.String())
							blobPlan := repoPlan.Blobs(src.ref, dst.t, location)

							toManifests, err := toRepo.Manifests(ctx)
							if err != nil {
								repoPlan.AddError(retrieverError{src: src.ref, dst: dst.ref, err: fmt.Errorf("unable to access destination image %s manifests: %v", src.ref, err)})
								continue
							}

							var mustCopyLayers bool
							switch {
							case o.Force:
								mustCopyLayers = true
							case src.ref.Registry == dst.ref.Registry && canonicalFrom.String() == canonicalTo.String():
								// if the source and destination repos are the same, we don't need to copy layers unless forced
							default:
								if _, err := toManifests.Get(ctx, srcDigest); err != nil {
									mustCopyLayers = true
									blobPlan.AlreadyExists(distribution.Descriptor{Digest: srcDigest})
								} else {
									glog.V(4).Infof("Manifest exists in %s, no need to copy layers without --force", dst.ref)
								}
							}

							toBlobs := toRepo.Blobs(ctx)

							if mustCopyLayers {
								// upload all the blobs
								srcBlobs := srcRepo.Blobs(ctx)

								// upload each manifest
								for _, srcManifest := range srcManifests {
									switch srcManifest.(type) {
									case *schema2.DeserializedManifest:
									case *manifestlist.DeserializedManifestList:
										// we do not need to upload layers in a manifestlist
										continue
									default:
										repoPlan.AddError(retrieverError{src: src.ref, dst: dst.ref, err: fmt.Errorf("the manifest type %T is not supported", srcManifest)})
										continue
									}
									for _, blob := range srcManifest.References() {
										blobPlan.Copy(blob, srcBlobs, toBlobs)
									}
								}
							}

							repoPlan.Manifests(dst.t).Copy(srcDigest, srcManifest, dst.tags, toManifests, toBlobs)
						}
					})
				}
			})
		})
	}
	for _, q := range registryWorkers {
		q.Done()
	}
	q.Done()

	plan.trim()
	plan.calculateStats()

	return plan, nil
}

func copyBlob(ctx context.Context, plan *workPlan, c *repositoryBlobCopy, blob distribution.Descriptor, force, skipMount bool, errOut io.Writer) error {
	// if we aren't forcing upload, check to see if the blob aleady exists
	if !force {
		_, err := c.to.Stat(ctx, blob.Digest)
		if err == nil {
			// blob exists, skip
			glog.V(5).Infof("Server reports blob exists %#v", blob)
			c.parent.parent.AssociateBlob(blob.Digest, c.parent.name)
			c.parent.ExpectBlob(blob.Digest)
			return nil
		}
		if err != distribution.ErrBlobUnknown {
			glog.V(5).Infof("Server was unable to check whether blob exists %s: %v", blob.Digest, err)
		}
	}

	var expectMount string
	var options []distribution.BlobCreateOption
	if !skipMount {
		if repo, ok := c.parent.parent.MountFrom(blob.Digest); ok {
			expectMount = repo
			canonicalFrom, err := reference.WithName(repo)
			if err != nil {
				return fmt.Errorf("unexpected error building named reference for %s: %v", repo, err)
			}
			blobSource, err := reference.WithDigest(canonicalFrom, blob.Digest)
			if err != nil {
				return fmt.Errorf("unexpected error building named digest: %v", err)
			}
			options = append(options, client.WithMountFrom(blobSource), WithDescriptor(blob))
		}
	}

	// if the object is small enough, put directly
	if blob.Size > 0 && blob.Size < 16384 {
		data, err := c.from.Get(ctx, blob.Digest)
		if err != nil {
			return fmt.Errorf("unable to push %s: failed to retrieve blob %s: %s", c.fromRef, blob.Digest, err)
		}
		desc, err := c.to.Put(ctx, blob.MediaType, data)
		if err != nil {
			return fmt.Errorf("unable to push %s: failed to upload blob %s: %s", c.fromRef, blob.Digest, err)
		}
		if desc.Digest != blob.Digest {
			return fmt.Errorf("unable to push %s: tried to copy blob %s and got back a different digest %s", c.fromRef, blob.Digest, desc.Digest)
		}
		plan.BytesCopied(blob.Size)
		return nil
	}

	w, err := c.to.Create(ctx, options...)
	// no-op
	if err == ErrAlreadyExists {
		glog.V(5).Infof("Blob already exists %#v", blob)
		return nil
	}

	// mount successful
	if ebm, ok := err.(distribution.ErrBlobMounted); ok {
		glog.V(5).Infof("Blob mounted %#v", blob)
		if ebm.From.Digest() != blob.Digest {
			return fmt.Errorf("unable to push %s: tried to mount blob %s source and got back a different digest %s", c.fromRef, blob.Digest, ebm.From.Digest())
		}
		switch c.destinationType {
		case DestinationS3:
			fmt.Fprintf(errOut, "mounted: s3://%s %s %s\n", c.toRef, blob.Digest, units.BytesSize(float64(blob.Size)))
		default:
			fmt.Fprintf(errOut, "mounted: %s %s %s\n", c.toRef, blob.Digest, units.BytesSize(float64(blob.Size)))
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("unable to upload blob %s to %s: %v", blob.Digest, c.toRef, err)
	}

	if len(expectMount) > 0 {
		fmt.Fprintf(errOut, "warning: Expected to mount %s from %s/%s but mount was ignored\n", blob.Digest, c.parent.parent.name, expectMount)
	}

	err = func() error {
		glog.V(5).Infof("Uploading blob %s", blob.Digest)
		defer w.Cancel(ctx)
		r, err := c.from.Open(ctx, blob.Digest)
		if err != nil {
			return fmt.Errorf("unable to open source layer %s to copy to %s: %v", blob.Digest, c.toRef, err)
		}
		defer r.Close()

		switch c.destinationType {
		case DestinationS3:
			fmt.Fprintf(errOut, "uploading: s3://%s %s %s\n", c.toRef, blob.Digest, units.BytesSize(float64(blob.Size)))
		default:
			fmt.Fprintf(errOut, "uploading: %s %s %s\n", c.toRef, blob.Digest, units.BytesSize(float64(blob.Size)))
		}

		n, err := w.ReadFrom(r)
		if err != nil {
			return fmt.Errorf("unable to copy layer %s to %s: %v", blob.Digest, c.toRef, err)
		}
		if n != blob.Size {
			fmt.Fprintf(errOut, "warning: Layer size mismatch for %s: had %d, wrote %d\n", blob.Digest, blob.Size, n)
		}
		if _, err := w.Commit(ctx, blob); err != nil {
			return err
		}
		plan.BytesCopied(n)
		return nil
	}()
	if err != nil {
		return fmt.Errorf("failed to commit blob %s from %s to %s: %v", blob.Digest, c.location, c.toRef, err)
	}
	return nil
}

func copyManifests(
	ctx context.Context,
	plan *repositoryManifestPlan,
	out io.Writer,
) []error {

	var errs []error
	ref, err := reference.WithName(plan.toRef.RepositoryName())
	if err != nil {
		return []error{fmt.Errorf("unable to create reference to repository %s: %v", plan.toRef, err)}
	}

	// upload and tag the manifest
	for srcDigest, tags := range plan.digestsToTags {
		srcManifest, ok := plan.parent.parent.parent.GetManifest(srcDigest)
		if !ok {
			panic(fmt.Sprintf("empty source manifest for %s", srcDigest))
		}
		for _, tag := range tags.List() {
			toDigest, err := imagemanifest.PutManifestInCompatibleSchema(ctx, srcManifest, tag, plan.to, plan.toBlobs, ref)
			if err != nil {
				errs = append(errs, fmt.Errorf("unable to push manifest to %s: %v", plan.toRef, err))
				continue
			}
			for _, desc := range srcManifest.References() {
				plan.parent.parent.AssociateBlob(desc.Digest, plan.parent.name)
			}
			switch plan.destinationType {
			case DestinationS3:
				fmt.Fprintf(out, "%s s3://%s:%s\n", toDigest, plan.toRef, tag)
			default:
				fmt.Fprintf(out, "%s %s:%s\n", toDigest, plan.toRef, tag)
			}
		}
	}
	// this is a pure manifest move, put the manifest by its id
	for digest := range plan.digestCopies {
		srcDigest := godigest.Digest(digest)
		srcManifest, ok := plan.parent.parent.parent.GetManifest(srcDigest)
		if !ok {
			panic(fmt.Sprintf("empty source manifest for %s", srcDigest))
		}
		toDigest, err := imagemanifest.PutManifestInCompatibleSchema(ctx, srcManifest, "", plan.to, plan.toBlobs, ref)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to push manifest to %s: %v", plan.toRef, err))
			continue
		}
		for _, desc := range srcManifest.References() {
			plan.parent.parent.AssociateBlob(desc.Digest, plan.parent.name)
		}
		switch plan.destinationType {
		case DestinationS3:
			fmt.Fprintf(out, "%s s3://%s\n", toDigest, plan.toRef)
		default:
			fmt.Fprintf(out, "%s %s\n", toDigest, plan.toRef)
		}
	}
	return errs
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
