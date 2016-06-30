package server

import (
	"fmt"
	"net/http"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
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

	opts, err := effectiveCreateOptions(options)
	if err != nil {
		return nil, err
	}
	if err := checkPendingCrossMountErrors(ctx, opts); err != nil {
		context.GetLogger(r.repo.ctx).Debugf("disabling cross-mount because of pending error: %v", err)
		options = append(options, guardCreateOptions{DisableCrossMount: true})
	} else {
		options = append(options, guardCreateOptions{})
	}

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

// Find out what the blob creation options are going to do by dry-running them
func effectiveCreateOptions(options []distribution.BlobCreateOption) (*storage.CreateOptions, error) {
	opts := &storage.CreateOptions{}
	for _, createOptions := range options {
		err := createOptions.Apply(opts)
		if err != nil {
			return nil, err
		}
	}
	return opts, nil
}

func checkPendingCrossMountErrors(ctx context.Context, opts *storage.CreateOptions) error {
	if !opts.Mount.ShouldMount {
		return nil
	}
	namespace, name, err := getNamespaceName(opts.Mount.From.Name())
	if err != nil {
		return err
	}
	return checkPendingErrors(context.GetLogger(ctx), ctx, namespace, name)
}

// guardCreateOptions ensures the expected options type is passed, and optionally disables cross mounting
type guardCreateOptions struct {
	DisableCrossMount bool
}

var _ distribution.BlobCreateOption = guardCreateOptions{}

func (f guardCreateOptions) Apply(v interface{}) error {
	opts, ok := v.(*storage.CreateOptions)
	if !ok {
		return fmt.Errorf("Unexpected create options: %#v", v)
	}
	if f.DisableCrossMount {
		opts.Mount.ShouldMount = false
	}
	return nil
}
