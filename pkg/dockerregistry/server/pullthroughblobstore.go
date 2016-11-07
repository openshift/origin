package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"

	"k8s.io/kubernetes/pkg/api/errors"

	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/importer"
)

// pullthroughBlobStore wraps a distribution.BlobStore and allows remote repositories to serve blobs from remote
// repositories.
type pullthroughBlobStore struct {
	distribution.BlobStore

	repo                       *repository
	digestToStore              map[string]distribution.BlobStore
	pullFromInsecureRegistries bool
}

var _ distribution.BlobStore = &pullthroughBlobStore{}

// Stat makes a local check for the blob, then falls through to the other servers referenced by
// the image stream and looks for those that have the layer.
func (r *pullthroughBlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	// check the local store for the blob
	desc, err := r.BlobStore.Stat(ctx, dgst)
	switch {
	case err == distribution.ErrBlobUnknown:
		// continue on to the code below and look up the blob in a remote store since it is not in
		// the local store
	case err != nil:
		context.GetLogger(ctx).Errorf("Failed to find blob %q: %#v", dgst.String(), err)
		fallthrough
	default:
		return desc, err
	}

	return r.remoteStat(ctx, dgst)
}

// remoteStat attempts to find requested blob in candidate remote repositories and if found, it updates
// digestToRepository store. ErrBlobUnknown will be returned if not found.
func (r *pullthroughBlobStore) remoteStat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	// look up the potential remote repositories that this blob could be part of (at this time,
	// we don't know which image in the image stream surfaced the content).
	is, err := r.repo.getImageStream()
	if err != nil {
		if errors.IsNotFound(err) || errors.IsForbidden(err) {
			return distribution.Descriptor{}, distribution.ErrBlobUnknown
		}
		context.GetLogger(ctx).Errorf("Error retrieving image stream for blob: %v", err)
		return distribution.Descriptor{}, err
	}

	r.pullFromInsecureRegistries = false

	if insecure, ok := is.Annotations[imageapi.InsecureRepositoryAnnotation]; ok {
		r.pullFromInsecureRegistries = insecure == "true"
	}

	var localRegistry string
	if local, err := imageapi.ParseDockerImageReference(is.Status.DockerImageRepository); err == nil {
		// TODO: normalize further?
		localRegistry = local.Registry
	}

	retriever := r.repo.importContext()
	cached := r.repo.cachedLayers.RepositoriesForDigest(dgst)

	// look at the first level of tagged repositories first
	search := identifyCandidateRepositories(is, localRegistry, true)
	if desc, err := r.findCandidateRepository(ctx, search, cached, dgst, retriever); err == nil {
		return desc, nil
	}

	// look at all other repositories tagged by the server
	secondary := identifyCandidateRepositories(is, localRegistry, false)
	for k := range search {
		delete(secondary, k)
	}
	if desc, err := r.findCandidateRepository(ctx, secondary, cached, dgst, retriever); err == nil {
		return desc, nil
	}

	return distribution.Descriptor{}, distribution.ErrBlobUnknown
}

// proxyStat attempts to locate the digest in the provided remote repository or returns an error. If the digest is found,
// r.digestToStore saves the store.
func (r *pullthroughBlobStore) proxyStat(ctx context.Context, retriever importer.RepositoryRetriever, ref imageapi.DockerImageReference, dgst digest.Digest) (distribution.Descriptor, error) {
	context.GetLogger(ctx).Infof("Trying to stat %q from %q", dgst, ref.Exact())
	repo, err := retriever.Repository(ctx, ref.RegistryURL(), ref.RepositoryName(), r.pullFromInsecureRegistries)
	if err != nil {
		context.GetLogger(ctx).Errorf("Error getting remote repository for image %q: %v", ref.Exact(), err)
		return distribution.Descriptor{}, err
	}
	pullthroughBlobStore := repo.Blobs(ctx)
	desc, err := pullthroughBlobStore.Stat(ctx, dgst)
	if err != nil {
		if err != distribution.ErrBlobUnknown {
			context.GetLogger(ctx).Errorf("Error getting pullthroughBlobStore for image %q: %v", ref.Exact(), err)
		}
		return distribution.Descriptor{}, err
	}

	r.digestToStore[dgst.String()] = pullthroughBlobStore
	return desc, nil
}

// ServeBlob attempts to serve the requested digest onto w, using a remote proxy store if necessary.
func (r *pullthroughBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, dgst digest.Digest) error {
	store, ok := r.digestToStore[dgst.String()]
	if !ok {
		return r.BlobStore.ServeBlob(ctx, w, req, dgst)
	}

	desc, err := store.Stat(ctx, dgst)
	if err != nil {
		context.GetLogger(ctx).Errorf("failed to stat digest %q: %v", dgst.String(), err)
		return err
	}

	remoteReader, err := store.Open(ctx, dgst)
	if err != nil {
		context.GetLogger(ctx).Errorf("failure to open remote store for digest %q: %v", dgst.String(), err)
		return err
	}
	defer remoteReader.Close()

	setResponseHeaders(w, desc.Size, desc.MediaType, dgst)

	context.GetLogger(ctx).Infof("serving blob %s of type %s %d bytes long", dgst.String(), desc.MediaType, desc.Size)
	http.ServeContent(w, req, desc.Digest.String(), time.Time{}, remoteReader)
	return nil
}

// Get attempts to fetch the requested blob by digest using a remote proxy store if necessary.
func (r *pullthroughBlobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	store, ok := r.digestToStore[dgst.String()]
	if ok {
		return store.Get(ctx, dgst)
	}

	data, originalErr := r.BlobStore.Get(ctx, dgst)
	if originalErr == nil {
		return data, nil
	}

	desc, err := r.remoteStat(ctx, dgst)
	if err != nil {
		context.GetLogger(ctx).Errorf("failed to stat blob %q in remote repositories: %v", dgst.String(), err)
		return nil, originalErr
	}
	store, ok = r.digestToStore[desc.Digest.String()]
	if !ok {
		return nil, originalErr
	}
	return store.Get(ctx, desc.Digest)
}

// findCandidateRepository looks in search for a particular blob, referring to previously cached items
func (r *pullthroughBlobStore) findCandidateRepository(ctx context.Context, search map[string]*imageapi.DockerImageReference, cachedLayers []string, dgst digest.Digest, retriever importer.RepositoryRetriever) (distribution.Descriptor, error) {
	// no possible remote locations to search, exit early
	if len(search) == 0 {
		return distribution.Descriptor{}, distribution.ErrBlobUnknown
	}

	// see if any of the previously located repositories containing this digest are in this
	// image stream
	for _, repo := range cachedLayers {
		ref, ok := search[repo]
		if !ok {
			continue
		}
		desc, err := r.proxyStat(ctx, retriever, *ref, dgst)
		if err != nil {
			delete(search, repo)
			continue
		}
		context.GetLogger(ctx).Infof("Found digest location from cache %q in %q", dgst, repo)
		return desc, nil
	}

	// search the remaining registries for this digest
	for repo, ref := range search {
		desc, err := r.proxyStat(ctx, retriever, *ref, dgst)
		if err != nil {
			continue
		}
		r.repo.cachedLayers.RememberDigest(dgst, r.repo.blobrepositorycachettl, repo)
		context.GetLogger(ctx).Infof("Found digest location by search %q in %q", dgst, repo)
		return desc, nil
	}

	return distribution.Descriptor{}, distribution.ErrBlobUnknown
}

// identifyCandidateRepositories returns a map of remote repositories referenced by this image stream.
func identifyCandidateRepositories(is *imageapi.ImageStream, localRegistry string, primary bool) map[string]*imageapi.DockerImageReference {
	// identify the canonical location of referenced registries to search
	search := make(map[string]*imageapi.DockerImageReference)
	for _, tagEvent := range is.Status.Tags {
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
			search[ref.AsRepository().Exact()] = &ref
		}
	}
	return search
}

// setResponseHeaders sets the appropriate content serving headers
func setResponseHeaders(w http.ResponseWriter, length int64, mediaType string, digest digest.Digest) {
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Docker-Content-Digest", digest.String())
	w.Header().Set("Etag", digest.String())
}
