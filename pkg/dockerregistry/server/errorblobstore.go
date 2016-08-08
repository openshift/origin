package server

import (
	"fmt"
	"net/http"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/reference"
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
	return r.store.Stat(WithRepository(ctx, r.repo), dgst)
}

func (r *errorBlobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return nil, err
	}
	return r.store.Get(WithRepository(ctx, r.repo), dgst)
}

func (r *errorBlobStore) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return nil, err
	}
	return r.store.Open(WithRepository(ctx, r.repo), dgst)
}

func (r *errorBlobStore) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return distribution.Descriptor{}, err
	}
	return r.store.Put(WithRepository(ctx, r.repo), mediaType, p)
}

func (r *errorBlobStore) Create(ctx context.Context, options ...distribution.BlobCreateOption) (distribution.BlobWriter, error) {
	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return nil, err
	}

	ctx = WithRepository(ctx, r.repo)

	opts, err := effectiveCreateOptions(options)
	if err != nil {
		return nil, err
	}
	err = checkPendingCrossMountErrors(ctx, opts)

	if err != nil {
		context.GetLogger(ctx).Infof("disabling cross-repo mount because of an error: %v", err)
		options = append(options, guardCreateOptions{DisableCrossMount: true})
	} else if !opts.Mount.ShouldMount {
		options = append(options, guardCreateOptions{DisableCrossMount: true})
	} else {
		context.GetLogger(ctx).Debugf("attempting cross-repo mount")
		options = append(options, statCrossMountCreateOptions{
			ctx:      ctx,
			destRepo: r.repo,
		})
	}

	return r.store.Create(ctx, options...)
}

func (r *errorBlobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return nil, err
	}
	return r.store.Resume(WithRepository(ctx, r.repo), id)
}

func (r *errorBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, dgst digest.Digest) error {
	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return err
	}
	return r.store.ServeBlob(WithRepository(ctx, r.repo), w, req, dgst)
}

func (r *errorBlobStore) Delete(ctx context.Context, dgst digest.Digest) error {
	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return err
	}
	return r.store.Delete(WithRepository(ctx, r.repo), dgst)
}

// checkPendingCrossMountErrors returns true if a cross-repo mount has been requested with given create
// options. If requested and there are pending authorization errors for source repository, the error will be
// returned. Cross-repo mount must not be allowed in case of error.
func checkPendingCrossMountErrors(ctx context.Context, opts *distribution.CreateOptions) error {
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
	opts, ok := v.(*distribution.CreateOptions)
	if !ok {
		return fmt.Errorf("Unexpected create options: %#v", v)
	}
	if f.DisableCrossMount {
		opts.Mount.ShouldMount = false
	}
	return nil
}

// statCrossMountCreateOptions ensures the expected options type is passed, and optionally pre-fills the cross-mount stat info
type statCrossMountCreateOptions struct {
	ctx      context.Context
	destRepo *repository
}

var _ distribution.BlobCreateOption = statCrossMountCreateOptions{}

func (f statCrossMountCreateOptions) Apply(v interface{}) error {
	opts, ok := v.(*distribution.CreateOptions)
	if !ok {
		return fmt.Errorf("Unexpected create options: %#v", v)
	}

	if !opts.Mount.ShouldMount {
		return nil
	}

	desc, err := statSourceRepository(f.ctx, f.destRepo, opts.Mount.From, opts.Mount.From.Digest())
	if err != nil {
		context.GetLogger(f.ctx).Infof("cannot mount blob %s from repository %s: %v - disabling cross-repo mount",
			opts.Mount.From.Digest().String(),
			opts.Mount.From.Name())
		opts.Mount.ShouldMount = false
		return nil
	}

	opts.Mount.Stat = &desc

	return nil
}

func statSourceRepository(
	ctx context.Context,
	destRepo *repository,
	sourceRepoName reference.Named,
	dgst digest.Digest,
) (desc distribution.Descriptor, err error) {
	upstreamRepo, err := dockerRegistry.Repository(ctx, sourceRepoName)
	if err != nil {
		return distribution.Descriptor{}, err
	}
	namespace, name, err := getNamespaceName(sourceRepoName.Name())
	if err != nil {
		return distribution.Descriptor{}, err
	}

	repo := *destRepo
	repo.namespace = namespace
	repo.name = name
	repo.Repository = upstreamRepo

	return repo.Blobs(ctx).Stat(ctx, dgst)
}
