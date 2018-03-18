package mirror

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/api/v2"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"

	units "github.com/docker/go-units"
	"github.com/docker/libtrust"
	"github.com/golang/glog"
	godigest "github.com/opencontainers/go-digest"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/registryclient"
	"github.com/openshift/origin/pkg/image/registryclient/dockercredentials"
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

		Experimental: This command is under active development and may change without notice.`)

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

type DestinationType string

var (
	DestinationRegistry DestinationType = "docker"
	DestinationS3       DestinationType = "s3"
)

type Mapping struct {
	Source      imageapi.DockerImageReference
	Destination imageapi.DockerImageReference
	Type        DestinationType
}

type pushOptions struct {
	Out, ErrOut io.Writer

	Filenames []string

	Mappings []Mapping
	OSFilter *regexp.Regexp

	FilterByOS string

	Insecure  bool
	SkipMount bool
	Force     bool

	AttemptS3BucketCopy []string
}

// schema2ManifestOnly specifically requests a manifest list first
var schema2ManifestOnly = distribution.WithManifestMediaTypes([]string{
	manifestlist.MediaTypeManifestList,
	schema2.MediaTypeManifest,
})

// NewCommandMirrorImage copies images from one location to another.
func NewCmdMirrorImage(name string, out, errOut io.Writer) *cobra.Command {
	o := &pushOptions{}

	cmd := &cobra.Command{
		Use:     "mirror SRC DST [DST ...]",
		Short:   "Mirror images from one repository to another",
		Long:    mirrorDesc,
		Example: fmt.Sprintf(mirrorExample, name+" mirror"),
		Run: func(c *cobra.Command, args []string) {
			o.Out = out
			o.ErrOut = errOut
			kcmdutil.CheckErr(o.Complete(args))
			kcmdutil.CheckErr(o.Run())
		},
	}

	flag := cmd.Flags()
	flag.BoolVar(&o.Insecure, "insecure", o.Insecure, "If true, connections may be made over HTTP")
	flag.BoolVar(&o.SkipMount, "skip-mount", o.SkipMount, "If true, always push layers instead of cross-mounting them")
	flag.StringVar(&o.FilterByOS, "filter-by-os", o.FilterByOS, "A regular expression to control which images are mirrored. Images will be passed as '<platform>/<architecture>[/<variant>]'.")
	flag.BoolVar(&o.Force, "force", o.Force, "If true, attempt to write all contents.")
	flag.StringSliceVar(&o.AttemptS3BucketCopy, "s3-source-bucket", o.AttemptS3BucketCopy, "A list of bucket/path locations on S3 that may contain already uploaded blobs. Add [store] to the end to use the Docker registry path convention.")
	flag.StringSliceVarP(&o.Filenames, "filename", "f", o.Filenames, "One or more files to read SRC=DST or SRC DST [DST ...] mappings from.")

	return cmd
}

func parseSource(ref string) (imageapi.DockerImageReference, error) {
	src, err := imageapi.ParseDockerImageReference(ref)
	if err != nil {
		return src, fmt.Errorf("%q is not a valid image reference: %v", ref, err)
	}
	if len(src.Tag) == 0 && len(src.ID) == 0 {
		return src, fmt.Errorf("you must specify a tag or digest for SRC")
	}
	return src, nil
}

func parseDestination(ref string) (imageapi.DockerImageReference, DestinationType, error) {
	dstType := DestinationRegistry
	switch {
	case strings.HasPrefix(ref, "s3://"):
		dstType = DestinationS3
		ref = strings.TrimPrefix(ref, "s3://")
	}
	dst, err := imageapi.ParseDockerImageReference(ref)
	if err != nil {
		return dst, dstType, fmt.Errorf("%q is not a valid image reference: %v", ref, err)
	}
	if len(dst.ID) != 0 {
		return dst, dstType, fmt.Errorf("you must specify a tag for DST or leave it blank to only push by digest")
	}
	return dst, dstType, nil
}

func parseArgs(args []string, overlap map[string]string) ([]Mapping, error) {
	var remainingArgs []string
	var mappings []Mapping
	for _, s := range args {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			remainingArgs = append(remainingArgs, s)
			continue
		}
		if len(parts[0]) == 0 || len(parts[1]) == 0 {
			return nil, fmt.Errorf("all arguments must be valid SRC=DST mappings")
		}
		src, err := parseSource(parts[0])
		if err != nil {
			return nil, err
		}
		dst, dstType, err := parseDestination(parts[1])
		if err != nil {
			return nil, err
		}
		if _, ok := overlap[dst.String()]; ok {
			return nil, fmt.Errorf("each destination tag may only be specified once: %s", dst.String())
		}
		overlap[dst.String()] = src.String()

		mappings = append(mappings, Mapping{Source: src, Destination: dst, Type: dstType})
	}

	switch {
	case len(remainingArgs) > 1 && len(mappings) == 0:
		src, err := parseSource(remainingArgs[0])
		if err != nil {
			return nil, err
		}
		for i := 1; i < len(remainingArgs); i++ {
			dst, dstType, err := parseDestination(remainingArgs[i])
			if err != nil {
				return nil, err
			}
			if _, ok := overlap[dst.String()]; ok {
				return nil, fmt.Errorf("each destination tag may only be specified once: %s", dst.String())
			}
			overlap[dst.String()] = src.String()
			mappings = append(mappings, Mapping{Source: src, Destination: dst, Type: dstType})
		}
	case len(remainingArgs) == 1 && len(mappings) == 0:
		return nil, fmt.Errorf("all arguments must be valid SRC=DST mappings, or you must specify one SRC argument and one or more DST arguments")
	}
	return mappings, nil
}

func parseFile(filename string, overlap map[string]string) ([]Mapping, error) {
	var fileMappings []Mapping
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	lineNumber := 0
	for s.Scan() {
		line := s.Text()
		lineNumber++

		// remove comments and whitespace
		if i := strings.Index(line, "#"); i != -1 {
			line = line[0:i]
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		args := strings.Split(line, " ")
		mappings, err := parseArgs(args, overlap)
		if err != nil {
			return nil, fmt.Errorf("file %s, line %d: %v", filename, lineNumber, err)
		}
		fileMappings = append(fileMappings, mappings...)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return fileMappings, nil
}

func (o *pushOptions) Complete(args []string) error {
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

	pattern := o.FilterByOS
	if len(pattern) > 0 {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("--filter-by-os was not a valid regular expression: %v", err)
		}
		o.OSFilter = re
	}

	return nil
}

type key struct {
	registry   string
	repository string
}

type destination struct {
	t    DestinationType
	ref  imageapi.DockerImageReference
	tags []string
}

type pushTargets map[key]destination

type destinations struct {
	ref     imageapi.DockerImageReference
	tags    map[string]pushTargets
	digests map[string]pushTargets
}

func (d destinations) mergeIntoDigests(srcDigest godigest.Digest, target pushTargets) {
	srcKey := srcDigest.String()
	current, ok := d.digests[srcKey]
	if !ok {
		d.digests[srcKey] = target
		return
	}
	for repo, dst := range target {
		existing, ok := current[repo]
		if !ok {
			current[repo] = dst
			continue
		}
		existing.tags = append(existing.tags, dst.tags...)
	}
}

type targetTree map[key]destinations

func buildTargetTree(mappings []Mapping) targetTree {
	tree := make(targetTree)
	for _, m := range mappings {
		srcKey := key{registry: m.Source.Registry, repository: m.Source.RepositoryName()}
		dstKey := key{registry: m.Destination.Registry, repository: m.Destination.RepositoryName()}

		src, ok := tree[srcKey]
		if !ok {
			src.ref = m.Source.AsRepository()
			src.digests = make(map[string]pushTargets)
			src.tags = make(map[string]pushTargets)
			tree[srcKey] = src
		}

		var current pushTargets
		if tag := m.Source.Tag; len(tag) != 0 {
			current = src.tags[tag]
			if current == nil {
				current = make(pushTargets)
				src.tags[tag] = current
			}
		} else {
			current = src.digests[m.Source.ID]
			if current == nil {
				current = make(pushTargets)
				src.digests[m.Source.ID] = current
			}
		}

		dst, ok := current[dstKey]
		if !ok {
			dst.ref = m.Destination.AsRepository()
			dst.t = m.Type
		}
		if len(m.Destination.Tag) > 0 {
			dst.tags = append(dst.tags, m.Destination.Tag)
		}
		current[dstKey] = dst
	}
	return tree
}

type retrieverError struct {
	src, dst imageapi.DockerImageReference
	err      error
}

func (e retrieverError) Error() string {
	return e.err.Error()
}

func (o *pushOptions) Repository(ctx apirequest.Context, context *registryclient.Context, creds auth.CredentialStore, t DestinationType, ref imageapi.DockerImageReference) (distribution.Repository, error) {
	switch t {
	case DestinationRegistry:
		toClient := context.WithCredentials(creds)
		return toClient.Repository(ctx, ref.DockerClientDefaults().RegistryURL(), ref.RepositoryName(), o.Insecure)
	case DestinationS3:
		driver := &s3Driver{
			Creds:    creds,
			CopyFrom: o.AttemptS3BucketCopy,
		}
		url := ref.DockerClientDefaults().RegistryURL()
		return driver.Repository(ctx, url, ref.RepositoryName(), o.Insecure)
	default:
		return nil, fmt.Errorf("unrecognized destination type %s", t)
	}
}

// includeDescriptor returns true if the provided manifest should be included.
func (o *pushOptions) includeDescriptor(d *manifestlist.ManifestDescriptor) bool {
	if o.OSFilter == nil {
		return true
	}
	if len(d.Platform.Variant) > 0 {
		return o.OSFilter.MatchString(fmt.Sprintf("%s/%s/%s", d.Platform.OS, d.Platform.Architecture, d.Platform.Variant))
	}
	return o.OSFilter.MatchString(fmt.Sprintf("%s/%s", d.Platform.OS, d.Platform.Architecture))
}

// ErrAlreadyExists may be returned by the blob Create function to indicate that the blob already exists.
var ErrAlreadyExists = fmt.Errorf("blob already exists in the target location")

func (o *pushOptions) Run() error {
	tree := buildTargetTree(o.Mappings)

	creds := dockercredentials.NewLocal()
	ctx := apirequest.NewContext()

	rt, err := rest.TransportFor(&rest.Config{})
	if err != nil {
		return err
	}
	insecureRT, err := rest.TransportFor(&rest.Config{TLSClientConfig: rest.TLSClientConfig{Insecure: true}})
	if err != nil {
		return err
	}
	srcClient := registryclient.NewContext(rt, insecureRT).WithCredentials(creds)
	toContext := registryclient.NewContext(rt, insecureRT).WithActions("pull", "push")

	var errs []error
	for _, src := range tree {
		srcRepo, err := srcClient.Repository(ctx, src.ref.DockerClientDefaults().RegistryURL(), src.ref.RepositoryName(), o.Insecure)
		if err != nil {
			errs = append(errs, retrieverError{err: fmt.Errorf("unable to connect to %s: %v", src.ref, err), src: src.ref})
			continue
		}

		manifests, err := srcRepo.Manifests(ctx)
		if err != nil {
			errs = append(errs, retrieverError{src: src.ref, err: fmt.Errorf("unable to access source image %s manifests: %v", src.ref, err)})
			continue
		}

		var tagErrs []retrieverError
		var digestErrs []retrieverError

		// convert source tags to digests
		for srcTag, pushTargets := range src.tags {
			desc, err := srcRepo.Tags(ctx).Get(ctx, srcTag)
			if err != nil {
				tagErrs = append(tagErrs, retrieverError{src: src.ref, err: fmt.Errorf("unable to retrieve source image %s by tag: %v", src.ref, err)})
				continue
			}
			srcDigest := desc.Digest
			glog.V(3).Infof("Resolved source image %s:%s to %s\n", src.ref, srcTag, srcDigest)
			src.mergeIntoDigests(srcDigest, pushTargets)
		}

		canonicalFrom := srcRepo.Named()

		for srcDigestString, pushTargets := range src.digests {
			// load the manifest
			srcDigest := godigest.Digest(srcDigestString)
			srcManifest, err := manifests.Get(ctx, godigest.Digest(srcDigest), schema2ManifestOnly)
			if err != nil {
				digestErrs = append(digestErrs, retrieverError{src: src.ref, err: fmt.Errorf("unable to retrieve source image %s manifest: %v", src.ref, err)})
				continue
			}

			// filter or load manifest list as appropriate
			srcManifests, srcManifest, srcDigest, err := processManifestList(ctx, srcDigest, srcManifest, manifests, src.ref, o.includeDescriptor)
			if err != nil {
				digestErrs = append(digestErrs, retrieverError{src: src.ref, err: err})
				continue
			}
			if len(srcManifests) == 0 {
				fmt.Fprintf(o.ErrOut, "info: Filtered all images from %s, skipping\n", src.ref)
				continue
			}

			for _, dst := range pushTargets {
				// if we are going to be using cross repository mount, get a token that covers the src
				if src.ref.Registry == dst.ref.Registry {
					toContext = toContext.WithScopes(auth.RepositoryScope{Repository: src.ref.RepositoryName(), Actions: []string{"pull"}})
				}

				toRepo, err := o.Repository(ctx, toContext, creds, dst.t, dst.ref)
				if err != nil {
					digestErrs = append(digestErrs, retrieverError{src: src.ref, dst: dst.ref, err: fmt.Errorf("unable to connect to %s: %v", dst.ref, err)})
					continue
				}

				canonicalTo := toRepo.Named()
				toManifests, err := toRepo.Manifests(ctx)
				if err != nil {
					digestErrs = append(digestErrs, retrieverError{src: src.ref, dst: dst.ref, err: fmt.Errorf("unable to access destination image %s manifests: %v", src.ref, err)})
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
					} else {
						glog.V(4).Infof("Manifest exists in %s, no need to copy layers without --force", dst.ref)
					}
				}

				if mustCopyLayers {
					if errs := uploadBlobs(ctx, dst, srcRepo, toRepo, srcManifests, src.ref, srcDigest, canonicalFrom, o.Force, o.SkipMount, o.ErrOut); len(errs) > 0 {
						digestErrs = append(digestErrs, errs...)
						continue
					}
				}

				if errs := uploadAndTagManifests(ctx, dst, srcManifest, src.ref, toManifests, o.Out, toRepo.Blobs(ctx), canonicalTo); len(errs) > 0 {
					digestErrs = append(digestErrs, errs...)
					continue
				}
			}
		}
		for _, err := range append(tagErrs, digestErrs...) {
			errs = append(errs, err)
		}
	}
	return kerrors.NewAggregate(errs)
}

func processManifestList(ctx apirequest.Context, srcDigest godigest.Digest, srcManifest distribution.Manifest, manifests distribution.ManifestService, ref imageapi.DockerImageReference, filterFn func(*manifestlist.ManifestDescriptor) bool) ([]distribution.Manifest, distribution.Manifest, godigest.Digest, error) {
	var srcManifests []distribution.Manifest
	switch t := srcManifest.(type) {
	case *manifestlist.DeserializedManifestList:
		manifestDigest := srcDigest
		manifestList := t

		filtered := make([]manifestlist.ManifestDescriptor, 0, len(t.Manifests))
		for _, manifest := range t.Manifests {
			if !filterFn(&manifest) {
				glog.V(5).Infof("Skipping image for %#v from %s", manifest.Platform, ref)
				continue
			}
			glog.V(5).Infof("Including image for %#v from %s", manifest.Platform, ref)
			filtered = append(filtered, manifest)
		}

		if len(filtered) == 0 {
			return nil, nil, "", nil
		}

		// if we're filtering the manifest list, update the source manifest and digest
		if len(filtered) != len(t.Manifests) {
			var err error
			t, err = manifestlist.FromDescriptors(filtered)
			if err != nil {
				return nil, nil, "", fmt.Errorf("unable to filter source image %s manifest list: %v", ref, err)
			}
			_, body, err := t.Payload()
			if err != nil {
				return nil, nil, "", fmt.Errorf("unable to filter source image %s manifest list (bad payload): %v", ref, err)
			}
			manifestList = t
			manifestDigest = srcDigest.Algorithm().FromBytes(body)
			glog.V(5).Infof("Filtered manifest list to new digest %s:\n%s", manifestDigest, body)
		}

		for i, manifest := range t.Manifests {
			childManifest, err := manifests.Get(ctx, manifest.Digest, distribution.WithManifestMediaTypes([]string{manifestlist.MediaTypeManifestList, schema2.MediaTypeManifest}))
			if err != nil {
				return nil, nil, "", fmt.Errorf("unable to retrieve source image %s manifest #%d from manifest list: %v", ref, i+1, err)
			}
			srcManifests = append(srcManifests, childManifest)
		}

		switch {
		case len(srcManifests) == 1:
			_, body, err := srcManifests[0].Payload()
			if err != nil {
				return nil, nil, "", fmt.Errorf("unable to convert source image %s manifest list to single manifest: %v", ref, err)
			}
			manifestDigest := srcDigest.Algorithm().FromBytes(body)
			glog.V(5).Infof("Used only one manifest from the list %s:\n%s", manifestDigest, body)
			return srcManifests, srcManifests[0], manifestDigest, nil
		default:
			return append(srcManifests, manifestList), manifestList, manifestDigest, nil
		}

	default:
		return []distribution.Manifest{srcManifest}, srcManifest, srcDigest, nil
	}
}

func uploadBlobs(
	ctx apirequest.Context,
	dst destination,
	srcRepo, toRepo distribution.Repository,
	srcManifests []distribution.Manifest,
	srcRef imageapi.DockerImageReference,
	srcDigest godigest.Digest,
	canonicalFrom reference.Named,
	force bool,
	skipMount bool,
	errOut io.Writer,
) []retrieverError {

	// upload all the blobs
	toBlobs := toRepo.Blobs(ctx)
	srcBlobs := srcRepo.Blobs(ctx)

	var errs []retrieverError

	// upload the each manifest
	for _, srcManifest := range srcManifests {
		switch srcManifest.(type) {
		case *schema2.DeserializedManifest:
		case *manifestlist.DeserializedManifestList:
			// we do not need to upload layers in a manifestlist
			continue
		default:
			errs = append(errs, retrieverError{src: srcRef, dst: dst.ref, err: fmt.Errorf("the manifest type %T is not supported", srcManifest)})
			continue
		}

		for _, blob := range srcManifest.References() {
			blobSource, err := reference.WithDigest(canonicalFrom, blob.Digest)
			if err != nil {
				errs = append(errs, retrieverError{src: srcRef, dst: dst.ref, err: fmt.Errorf("unexpected error building named digest: %v", err)})
				continue
			}

			// if we aren't forcing upload, skip the blob copy
			if !force {
				_, err := toBlobs.Stat(ctx, blob.Digest)
				if err == nil {
					// blob exists, skip
					glog.V(5).Infof("Server reports blob exists %#v", blob)
					continue
				}
				if err != distribution.ErrBlobUnknown {
					glog.V(5).Infof("Server was unable to check whether blob exists %s: %v", blob.Digest, err)
				}
			}

			var options []distribution.BlobCreateOption
			if !skipMount {
				options = append(options, client.WithMountFrom(blobSource), WithDescriptor(blob))
			}
			w, err := toBlobs.Create(ctx, options...)
			// no-op
			if err == ErrAlreadyExists {
				glog.V(5).Infof("Blob already exists %#v", blob)
				continue
			}
			// mount successful
			if ebm, ok := err.(distribution.ErrBlobMounted); ok {
				glog.V(5).Infof("Blob mounted %#v", blob)
				if ebm.From.Digest() != blob.Digest {
					errs = append(errs, retrieverError{src: srcRef, dst: dst.ref, err: fmt.Errorf("unable to push %s: tried to mount blob %s src source and got back a different digest %s", srcRef, blob.Digest, ebm.From.Digest())})
					break
				}
				continue
			}
			if err != nil {
				errs = append(errs, retrieverError{src: srcRef, dst: dst.ref, err: fmt.Errorf("unable to upload blob %s to %s: %v", blob.Digest, dst.ref, err)})
				break
			}

			err = func() error {
				glog.V(5).Infof("Uploading blob %s", blob.Digest)
				defer w.Cancel(ctx)
				r, err := srcBlobs.Open(ctx, blob.Digest)
				if err != nil {
					return fmt.Errorf("unable to open source layer %s to copy to %s: %v", blob.Digest, dst.ref, err)
				}
				defer r.Close()

				switch dst.t {
				case DestinationS3:
					fmt.Fprintf(errOut, "uploading: s3://%s %s %s\n", dst.ref, blob.Digest, units.BytesSize(float64(blob.Size)))
				default:
					fmt.Fprintf(errOut, "uploading: %s %s %s\n", dst.ref, blob.Digest, units.BytesSize(float64(blob.Size)))
				}

				n, err := w.ReadFrom(r)
				if err != nil {
					return fmt.Errorf("unable to copy layer %s to %s: %v", blob.Digest, dst.ref, err)
				}
				if n != blob.Size {
					fmt.Fprintf(errOut, "warning: Layer size mismatch for %s: had %d, wrote %d\n", blob.Digest, blob.Size, n)
				}
				_, err = w.Commit(ctx, blob)
				return err
			}()
			if err != nil {
				_, srcBody, _ := srcManifest.Payload()
				srcManifestDigest := godigest.Canonical.FromBytes(srcBody)
				if srcManifestDigest == srcDigest {
					errs = append(errs, retrieverError{src: srcRef, dst: dst.ref, err: fmt.Errorf("failed to commit blob %s from manifest %s to %s: %v", blob.Digest, srcManifestDigest, dst.ref, err)})
				} else {
					errs = append(errs, retrieverError{src: srcRef, dst: dst.ref, err: fmt.Errorf("failed to commit blob %s from manifest %s in manifest list %s to %s: %v", blob.Digest, srcManifestDigest, srcDigest, dst.ref, err)})
				}
				break
			}
		}
	}
	return errs
}

func uploadAndTagManifests(
	ctx apirequest.Context,
	dst destination,
	srcManifest distribution.Manifest,
	srcRef imageapi.DockerImageReference,
	toManifests distribution.ManifestService,
	out io.Writer,
	// supports schema2->schema1 downconversion
	blobs distribution.BlobService,
	ref reference.Named,
) []retrieverError {
	var errs []retrieverError

	// upload and tag the manifest
	for _, tag := range dst.tags {
		toDigest, err := putManifestInCompatibleSchema(ctx, srcManifest, tag, toManifests, blobs, ref)
		if err != nil {
			errs = append(errs, retrieverError{src: srcRef, dst: dst.ref, err: fmt.Errorf("unable to push manifest to %s: %v", dst.ref, err)})
			continue
		}
		switch dst.t {
		case DestinationS3:
			fmt.Fprintf(out, "%s s3://%s:%s\n", toDigest, dst.ref, tag)
		default:
			fmt.Fprintf(out, "%s %s:%s\n", toDigest, dst.ref, tag)
		}
	}
	if len(dst.tags) != 0 {
		return errs
	}

	// this is a pure manifest move, put the manifest by its id
	toDigest, err := putManifestInCompatibleSchema(ctx, srcManifest, "latest", toManifests, blobs, ref)
	if err != nil {
		errs = append(errs, retrieverError{src: srcRef, dst: dst.ref, err: fmt.Errorf("unable to push manifest to %s: %v", dst.ref, err)})
		return errs
	}
	switch dst.t {
	case DestinationS3:
		fmt.Fprintf(out, "%s s3://%s\n", toDigest, dst.ref)
	default:
		fmt.Fprintf(out, "%s %s\n", toDigest, dst.ref)
	}
	return errs
}

// TDOO: remove when quay.io switches to v2 schema
func putManifestInCompatibleSchema(
	ctx apirequest.Context,
	srcManifest distribution.Manifest,
	tag string,
	toManifests distribution.ManifestService,
	// supports schema2 -> schema1 downconversion
	blobs distribution.BlobService,
	ref reference.Named,
) (godigest.Digest, error) {

	toDigest, err := toManifests.Put(ctx, srcManifest, distribution.WithTag(tag))
	if err == nil {
		return toDigest, nil
	}
	errs, ok := err.(errcode.Errors)
	if !ok || len(errs) == 0 {
		return toDigest, err
	}
	errcode, ok := errs[0].(errcode.Error)
	if !ok || errcode.ErrorCode() != v2.ErrorCodeManifestInvalid {
		return toDigest, err
	}
	// try downconverting to v2-schema1
	schema2Manifest, ok := srcManifest.(*schema2.DeserializedManifest)
	if !ok {
		return toDigest, err
	}
	ref, tagErr := reference.WithTag(ref, tag)
	if tagErr != nil {
		return toDigest, err
	}
	schema1Manifest, convertErr := convertToSchema1(ctx, blobs, schema2Manifest, ref)
	if convertErr != nil {
		return toDigest, err
	}
	return toManifests.Put(ctx, schema1Manifest, distribution.WithTag(tag))
}

// TDOO: remove when quay.io switches to v2 schema
func convertToSchema1(ctx apirequest.Context, blobs distribution.BlobService, schema2Manifest *schema2.DeserializedManifest, ref reference.Named) (distribution.Manifest, error) {
	targetDescriptor := schema2Manifest.Target()
	configJSON, err := blobs.Get(ctx, targetDescriptor.Digest)
	if err != nil {
		return nil, err
	}
	trustKey, err := loadPrivateKey()
	if err != nil {
		return nil, err
	}
	builder := schema1.NewConfigManifestBuilder(blobs, trustKey, ref, configJSON)
	for _, d := range schema2Manifest.Layers {
		if err := builder.AppendReference(d); err != nil {
			return nil, err
		}
	}
	manifest, err := builder.Build(ctx)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

var (
	privateKeyLock sync.Mutex
	privateKey     libtrust.PrivateKey
)

// TDOO: remove when quay.io switches to v2 schema
func loadPrivateKey() (libtrust.PrivateKey, error) {
	privateKeyLock.Lock()
	defer privateKeyLock.Unlock()
	if privateKey != nil {
		return privateKey, nil
	}
	trustKey, err := libtrust.GenerateECP256PrivateKey()
	if err != nil {
		return nil, err
	}
	privateKey = trustKey
	return privateKey, nil
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
