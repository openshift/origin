package server

import (
	"net/http"
	"sort"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/api/errcode"
	disterrors "github.com/docker/distribution/registry/api/v2"

	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/importer"
)

// BlobGetterService combines the operations to access and read blobs.
type BlobGetterService interface {
	distribution.BlobStatter
	distribution.BlobProvider
	distribution.BlobServer
}

type ImageStreamGetter func() (*imageapi.ImageStream, error)

// remoteBlobGetterService implements BlobGetterService and allows to serve blobs from remote
// repositories.
type remoteBlobGetterService struct {
	namespace           string
	name                string
	cacheTTL            time.Duration
	getImageStream      ImageStreamGetter
	isSecretsNamespacer osclient.ImageStreamSecretsNamespacer
	cachedLayers        digestToRepositoryCache
	digestToStore       map[string]distribution.BlobStore
}

var _ BlobGetterService = &remoteBlobGetterService{}

// NewBlobGetterService returns a getter for remote blobs. Its cache will be shared among different middleware
// wrappers, which is a must at least for stat calls made on manifest's dependencies during its verification.
func NewBlobGetterService(
	namespace, name string,
	cacheTTL time.Duration,
	imageStreamGetter ImageStreamGetter,
	isSecretsNamespacer osclient.ImageStreamSecretsNamespacer,
	cachedLayers digestToRepositoryCache,
) BlobGetterService {
	return &remoteBlobGetterService{
		namespace:           namespace,
		name:                name,
		getImageStream:      imageStreamGetter,
		isSecretsNamespacer: isSecretsNamespacer,
		cacheTTL:            cacheTTL,
		cachedLayers:        cachedLayers,
		digestToStore:       make(map[string]distribution.BlobStore),
	}
}

// imagePullthroughSpec contains a reference of remote image to pull associated with an insecure flag for the
// corresponding registry.
type imagePullthroughSpec struct {
	dockerImageReference *imageapi.DockerImageReference
	insecure             bool
}

// Stat provides metadata about a blob identified by the digest. If the
// blob is unknown to the describer, ErrBlobUnknown will be returned.
func (rbgs *remoteBlobGetterService) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	context.GetLogger(ctx).Debugf("(*remoteBlobGetterService).Stat: starting with dgst=%s", dgst.String())
	// look up the potential remote repositories that this blob could be part of (at this time,
	// we don't know which image in the image stream surfaced the content).
	is, err := rbgs.getImageStream()
	if err != nil {
		if t, ok := err.(errcode.Error); ok && t.ErrorCode() == disterrors.ErrorCodeNameUnknown {
			return distribution.Descriptor{}, distribution.ErrBlobUnknown
		}
		return distribution.Descriptor{}, err
	}

	var localRegistry string
	if local, err := imageapi.ParseDockerImageReference(is.Status.DockerImageRepository); err == nil {
		// TODO: normalize further?
		localRegistry = local.Registry
	}

	retriever := getImportContext(ctx, rbgs.isSecretsNamespacer, rbgs.namespace, rbgs.name)
	cached := rbgs.cachedLayers.RepositoriesForDigest(dgst)

	// look at the first level of tagged repositories first
	repositoryCandidates, search := identifyCandidateRepositories(is, localRegistry, true)
	if desc, err := rbgs.findCandidateRepository(ctx, repositoryCandidates, search, cached, dgst, retriever); err == nil {
		return desc, nil
	}

	// look at all other repositories tagged by the server
	repositoryCandidates, secondary := identifyCandidateRepositories(is, localRegistry, false)
	for k := range search {
		delete(secondary, k)
	}
	if desc, err := rbgs.findCandidateRepository(ctx, repositoryCandidates, secondary, cached, dgst, retriever); err == nil {
		return desc, nil
	}

	return distribution.Descriptor{}, distribution.ErrBlobUnknown
}

func (rbgs *remoteBlobGetterService) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	context.GetLogger(ctx).Debugf("(*remoteBlobGetterService).Open: starting with dgst=%s", dgst.String())
	store, ok := rbgs.digestToStore[dgst.String()]
	if ok {
		return store.Open(ctx, dgst)
	}

	desc, err := rbgs.Stat(ctx, dgst)
	if err != nil {
		context.GetLogger(ctx).Errorf("Open: failed to stat blob %q in remote repositories: %v", dgst.String(), err)
		return nil, err
	}

	store, ok = rbgs.digestToStore[desc.Digest.String()]
	if !ok {
		return nil, distribution.ErrBlobUnknown
	}

	return store.Open(ctx, desc.Digest)
}

func (rbgs *remoteBlobGetterService) ServeBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, dgst digest.Digest) error {
	context.GetLogger(ctx).Debugf("(*remoteBlobGetterService).ServeBlob: starting with dgst=%s", dgst.String())
	store, ok := rbgs.digestToStore[dgst.String()]
	if ok {
		return store.ServeBlob(ctx, w, req, dgst)
	}

	desc, err := rbgs.Stat(ctx, dgst)
	if err != nil {
		context.GetLogger(ctx).Errorf("ServeBlob: failed to stat blob %q in remote repositories: %v", dgst.String(), err)
		return err
	}

	store, ok = rbgs.digestToStore[desc.Digest.String()]
	if !ok {
		return distribution.ErrBlobUnknown
	}

	return store.ServeBlob(ctx, w, req, desc.Digest)
}

// proxyStat attempts to locate the digest in the provided remote repository or returns an error. If the digest is found,
// rbgs.digestToStore saves the store.
func (rbgs *remoteBlobGetterService) proxyStat(
	ctx context.Context,
	retriever importer.RepositoryRetriever,
	spec *imagePullthroughSpec,
	dgst digest.Digest,
) (distribution.Descriptor, error) {
	ref := spec.dockerImageReference
	insecureNote := ""
	if spec.insecure {
		insecureNote = " with a fall-back to insecure transport"
	}
	context.GetLogger(ctx).Infof("Trying to stat %q from %q%s", dgst, ref.AsRepository().Exact(), insecureNote)
	repo, err := retriever.Repository(ctx, ref.RegistryURL(), ref.RepositoryName(), spec.insecure)
	if err != nil {
		context.GetLogger(ctx).Errorf("Error getting remote repository for image %q: %v", ref.AsRepository().Exact(), err)
		return distribution.Descriptor{}, err
	}

	pullthroughBlobStore := repo.Blobs(ctx)
	desc, err := pullthroughBlobStore.Stat(ctx, dgst)
	if err != nil {
		if err != distribution.ErrBlobUnknown {
			context.GetLogger(ctx).Errorf("Error statting blob %s in remote repository %q: %v", dgst, ref.AsRepository().Exact(), err)
		}
		return distribution.Descriptor{}, err
	}

	rbgs.digestToStore[dgst.String()] = pullthroughBlobStore
	return desc, nil
}

// Get attempts to fetch the requested blob by digest using a remote proxy store if necessary.
func (rbgs *remoteBlobGetterService) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	context.GetLogger(ctx).Debugf("(*remoteBlobGetterService).Get: starting with dgst=%s", dgst.String())
	store, ok := rbgs.digestToStore[dgst.String()]
	if ok {
		return store.Get(ctx, dgst)
	}

	desc, err := rbgs.Stat(ctx, dgst)
	if err != nil {
		context.GetLogger(ctx).Errorf("Get: failed to stat blob %q in remote repositories: %v", dgst.String(), err)
		return nil, err
	}

	store, ok = rbgs.digestToStore[desc.Digest.String()]
	if !ok {
		return nil, distribution.ErrBlobUnknown
	}

	return store.Get(ctx, desc.Digest)
}

// findCandidateRepository looks in search for a particular blob, referring to previously cached items
func (rbgs *remoteBlobGetterService) findCandidateRepository(
	ctx context.Context,
	repositoryCandidates []string,
	search map[string]imagePullthroughSpec,
	cachedLayers []string,
	dgst digest.Digest,
	retriever importer.RepositoryRetriever,
) (distribution.Descriptor, error) {
	// no possible remote locations to search, exit early
	if len(search) == 0 {
		return distribution.Descriptor{}, distribution.ErrBlobUnknown
	}

	// see if any of the previously located repositories containing this digest are in this
	// image stream
	for _, repo := range cachedLayers {
		spec, ok := search[repo]
		if !ok {
			continue
		}
		desc, err := rbgs.proxyStat(ctx, retriever, &spec, dgst)
		if err != nil {
			delete(search, repo)
			continue
		}
		context.GetLogger(ctx).Infof("Found digest location from cache %q in %q", dgst, repo)
		return desc, nil
	}

	// search the remaining registries for this digest
	for _, repo := range repositoryCandidates {
		spec, ok := search[repo]
		if !ok {
			continue
		}
		desc, err := rbgs.proxyStat(ctx, retriever, &spec, dgst)
		if err != nil {
			continue
		}
		rbgs.cachedLayers.RememberDigest(dgst, rbgs.cacheTTL, repo)
		context.GetLogger(ctx).Infof("Found digest location by search %q in %q", dgst, repo)
		return desc, nil
	}

	return distribution.Descriptor{}, distribution.ErrBlobUnknown
}

type byInsecureFlag struct {
	repositories []string
	specs        []*imagePullthroughSpec
}

func (by *byInsecureFlag) Len() int {
	if len(by.specs) < len(by.repositories) {
		return len(by.specs)
	}
	return len(by.repositories)
}
func (by *byInsecureFlag) Swap(i, j int) {
	by.repositories[i], by.repositories[j] = by.repositories[j], by.repositories[i]
	by.specs[i], by.specs[j] = by.specs[j], by.specs[i]
}
func (by *byInsecureFlag) Less(i, j int) bool {
	if by.specs[i].insecure == by.specs[j].insecure {
		switch {
		case by.repositories[i] < by.repositories[j]:
			return true
		case by.repositories[i] > by.repositories[j]:
			return false
		default:
			return by.specs[i].dockerImageReference.Exact() < by.specs[j].dockerImageReference.Exact()
		}
	}
	return !by.specs[i].insecure
}

// identifyCandidateRepositories returns a list of remote repository names sorted from the best candidate to
// the worst and a map of remote repositories referenced by this image stream. The best candidate is a secure
// one. The worst allows for insecure transport.
func identifyCandidateRepositories(
	is *imageapi.ImageStream,
	localRegistry string,
	primary bool,
) ([]string, map[string]imagePullthroughSpec) {
	insecureByDefault := false
	if insecure, ok := is.Annotations[imageapi.InsecureRepositoryAnnotation]; ok {
		insecureByDefault = insecure == "true"
	}

	// maps registry to insecure flag
	insecureRegistries := make(map[string]bool)

	// identify the canonical location of referenced registries to search
	search := make(map[string]*imageapi.DockerImageReference)
	for tag, tagEvent := range is.Status.Tags {
		var candidates []imageapi.TagEvent
		if primary {
			if len(tagEvent.Items) == 0 {
				continue
			}
			candidates = tagEvent.Items[:1]
		} else {
			if len(tagEvent.Items) <= 1 {
				continue
			}
			candidates = tagEvent.Items[1:]
		}
		for _, event := range candidates {
			ref, err := imageapi.ParseDockerImageReference(event.DockerImageReference)
			if err != nil {
				continue
			}
			// skip anything that matches the innate registry
			// TODO: there may be a better way to make this determination
			if len(localRegistry) != 0 && localRegistry == ref.Registry {
				continue
			}
			ref = ref.DockerClientDefaults()
			insecure := insecureByDefault
			if tagRef, ok := is.Spec.Tags[tag]; ok {
				insecure = insecureByDefault || tagRef.ImportPolicy.Insecure
			}
			if is := insecureRegistries[ref.Registry]; !is && insecure {
				insecureRegistries[ref.Registry] = insecure
			}

			search[ref.AsRepository().Exact()] = &ref
		}
	}

	repositories := make([]string, 0, len(search))
	results := make(map[string]imagePullthroughSpec)
	specs := []*imagePullthroughSpec{}
	for repo, ref := range search {
		repositories = append(repositories, repo)
		// accompany the reference with corresponding registry's insecure flag
		spec := imagePullthroughSpec{
			dockerImageReference: ref,
			insecure:             insecureRegistries[ref.Registry],
		}
		results[repo] = spec
		specs = append(specs, &spec)
	}

	sort.Sort(&byInsecureFlag{repositories: repositories, specs: specs})

	return repositories, results
}

// pullInsecureByDefault returns true if the given repository or repository's tag allows for insecure
// transport.
func pullInsecureByDefault(isGetter ImageStreamGetter, tag string) bool {
	insecureByDefault := false

	is, err := isGetter()
	if err != nil {
		return insecureByDefault
	}

	if insecure, ok := is.Annotations[imageapi.InsecureRepositoryAnnotation]; ok {
		insecureByDefault = insecure == "true"
	}

	if insecureByDefault || len(tag) == 0 {
		return insecureByDefault
	}

	tagReference, ok := is.Spec.Tags[tag]
	return ok && tagReference.ImportPolicy.Insecure
}
