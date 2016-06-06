package server

import (
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"

	"k8s.io/kubernetes/pkg/api/errors"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

const (
	StorageGetContentNamespace = "openshift.storage.getcontent.namespace"
	StorageGetContentName      = "openshift.storage.getcontent.name"
)

// Package server wraps repository objects of docker/distribution upstream.
// Module forwards requests between local repositories in order to
// serve requested blob living in other repository referenced in
// corresponding image stream.
type localBlobStore struct {
	distribution.BlobStore

	repo *repository
}

var _ distribution.BlobStore = &localBlobStore{}

func (bs *localBlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	desc, err := bs.BlobStore.Stat(ctx, dgst)
	switch {
	case err == distribution.ErrBlobUnknown:
		// continue on to the code below and look up the blob in a local store
	case err != nil:
		context.GetLogger(bs.repo.ctx).Errorf("Failed to find blob %s: %#v", dgst.String(), err)
		fallthrough
	default:
		return desc, err
	}

	is, err := bs.repo.getImageStream()
	if err != nil {
		if errors.IsNotFound(err) {
			return distribution.Descriptor{}, distribution.ErrBlobUnknown
		}
		context.GetLogger(bs.repo.ctx).Errorf("Error retrieving image stream for blob: %s", err)

		if errors.IsForbidden(err) {
			return distribution.Descriptor{}, distribution.ErrBlobUnknown
		}
		return distribution.Descriptor{}, err
	}

	cached := bs.repo.cachedLayers.RepositoriesForDigest(dgst)

	// look at the first level of tagged repositories first
	search := bs.identifyCandidateRepositories(is, true)
	if desc, err := bs.findCandidateRepository(ctx, search, cached, dgst); err == nil {
		return desc, nil
	}

	// look at all other repositories tagged by the server
	secondary := bs.identifyCandidateRepositories(is, false)
	for k := range search {
		delete(secondary, k)
	}
	if desc, err := bs.findCandidateRepository(ctx, secondary, cached, dgst); err == nil {
		return desc, nil
	}

	return distribution.Descriptor{}, distribution.ErrBlobUnknown
}

func (bs *localBlobStore) crossStat(ctx context.Context, ref imageapi.DockerImageReference, dgst digest.Digest) (distribution.Descriptor, error) {
	statCtx := context.WithValue(ctx, StorageGetContentNamespace, ref.Namespace)
	statCtx = context.WithValue(statCtx, StorageGetContentName, ref.Name)

	desc, err := bs.BlobStore.Stat(statCtx, dgst)
	if err != nil {
		if err != distribution.ErrBlobUnknown {
			context.GetLogger(bs.repo.ctx).Errorf("Unable to stat %s from %q: %v", dgst, ref.Exact(), err)
		}
		return distribution.Descriptor{}, err
	}

	return desc, nil
}

// TODO(legion): Merge this function with pullthroughblobstore.
func (bs *localBlobStore) identifyCandidateRepositories(is *imageapi.ImageStream, primary bool) map[string]*imageapi.DockerImageReference {
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
			if bs.repo.registryAddr != ref.Registry {
				continue
			}
			ref = ref.DockerClientDefaults()

			// Use different key than pullthrough to avoid any intersections.
			search[ref.RepositoryName()] = &ref
		}
	}
	return search
}

// TODO(legion): Merge this function with pullthroughblobstore.
func (bs *localBlobStore) findCandidateRepository(ctx context.Context, search map[string]*imageapi.DockerImageReference, cachedLayers []string, dgst digest.Digest) (distribution.Descriptor, error) {
	if len(search) == 0 {
		return distribution.Descriptor{}, distribution.ErrBlobUnknown
	}

	for _, repo := range cachedLayers {
		ref, ok := search[repo]
		if !ok {
			continue
		}
		desc, err := bs.crossStat(ctx, *ref, dgst)
		if err != nil {
			delete(search, repo)
			continue
		}
		context.GetLogger(bs.repo.ctx).Infof("Found digest location from cache %s in %s", dgst, repo)
		return desc, nil
	}

	for reponame, ref := range search {
		desc, err := bs.crossStat(ctx, *ref, dgst)
		if err != nil {
			continue
		}
		bs.repo.cachedLayers.RememberDigest(dgst, reponame)
		context.GetLogger(bs.repo.ctx).Infof("Found digest location by search %s in %s", dgst, reponame)
		return desc, nil
	}
	return distribution.Descriptor{}, distribution.ErrBlobUnknown
}
