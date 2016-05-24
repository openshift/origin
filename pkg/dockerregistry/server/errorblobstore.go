package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/storage"
)

// errorBlobStore wraps a distribution.BlobStore for a particular repo.
// before delegating, it ensures auth completed and there were no errors relevant to the repo.
type errorBlobStore struct {
	store distribution.BlobStore
	repo  *repository
}

var _ distribution.BlobStore = &errorBlobStore{}

func (r *errorBlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return distribution.Descriptor{}, err
	}
	return r.store.Stat(ctx, dgst)
}

func (r *errorBlobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return nil, err
	}
	return r.store.Get(ctx, dgst)
}

func (r *errorBlobStore) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return nil, err
	}
	return r.store.Open(ctx, dgst)
}

func (r *errorBlobStore) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return distribution.Descriptor{}, err
	}
	return r.store.Put(ctx, mediaType, p)
}

func (r *errorBlobStore) Create(ctx context.Context, options ...distribution.BlobCreateOption) (distribution.BlobWriter, error) {
	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return nil, err
	}

	options = append(options, storage.WithRepositoryMiddlewareWrapper(
		func(ctx context.Context, repo distribution.Repository, name reference.Named) (distribution.Repository, error) {
			context.GetLogger(r.repo.ctx).Debugf("(*errorBlobStore).Create: called injected middleware wrapper function")
			nameParts := strings.SplitN(name.Name(), "/", 2)
			if len(nameParts) != 2 {
				return nil, fmt.Errorf("invalid repository name %q: it must be of the format <project>/<name>", repo.Named().Name())
			}
			middleware := *r.repo
			middleware.Repository = repo
			middleware.namespace = nameParts[0]
			middleware.name = nameParts[1]
			context.GetLogger(r.repo.ctx).Infof("(*errorBlobStore).Create: returning new middleware for repository=%s", middleware.Name())
			return &middleware, nil
		}))

	return r.store.Create(ctx, options...)
}

func (r *errorBlobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return nil, err
	}
	return r.store.Resume(ctx, id)
}

func (r *errorBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, dgst digest.Digest) error {
	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return err
	}
	return r.store.ServeBlob(ctx, w, req, dgst)
}

func (r *errorBlobStore) Delete(ctx context.Context, dgst digest.Digest) error {
	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return err
	}
	return r.store.Delete(ctx, dgst)
}
