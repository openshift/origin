package server

import (
	"fmt"
	"net/http"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/reference"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"

	imageapi "github.com/openshift/origin/pkg/image/api"
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
	var desc distribution.Descriptor

	if err := r.repo.checkPendingErrors(ctx); err != nil {
		return nil, err
	}

	ctx = WithRepository(ctx, r.repo)
	var pullthroughSourceImageReference *imageapi.DockerImageReference

	opts, err := effectiveCreateOptions(options)
	if err != nil {
		return nil, err
	}
	err = checkPendingCrossMountErrors(ctx, opts)
	if err == nil && opts.Mount.ShouldMount {
		context.GetLogger(ctx).Debugf("checking for presence of blob %s in a source repository %s", opts.Mount.From.Digest().String(), opts.Mount.From.Name())
		desc, pullthroughSourceImageReference, err = statSourceRepository(ctx, r.repo, opts.Mount.From, opts.Mount.From.Digest())
	}
	if err == nil && pullthroughSourceImageReference != nil {
		ref := pullthroughSourceImageReference.MostSpecific()
		context.GetLogger(ctx).Debugf("trying to tag source image %s into image stream %s", ref.Exact(), r.repo.Named().Name())
		err = tagPullthroughSourceImageInTargetRepository(ctx, &ref, r.repo, opts.Mount.From.Digest())
	}

	if err != nil {
		context.GetLogger(ctx).Infof("disabling cross-repo mount because of an error: %v", err)
		options = append(options, guardCreateOptions{DisableCrossMount: true})
	} else if !opts.Mount.ShouldMount {
		context.GetLogger(ctx).Infof("ensuring cross-repo is disabled: %v", err)
		options = append(options, guardCreateOptions{DisableCrossMount: true})
	} else {
		context.GetLogger(ctx).Debugf("attempting cross-repo mount")
		options = append(options, statCrossMountCreateOptions{desc: desc})
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
	desc distribution.Descriptor
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

	opts.Mount.Stat = &f.desc

	return nil
}

// statSourceRepository founds a blob in the source repository of cross-repo mount and returns its descriptor
// if found. If the blob is not stored locally but it's available in remote repository, the
// pullthroughSourceImageReference output parameter will be set to contain a reference of an image containing
// it.
func statSourceRepository(
	ctx context.Context,
	destRepo *repository,
	sourceRepoName reference.Named,
	dgst digest.Digest,
) (desc distribution.Descriptor, pullthroughSourceImageReference *imageapi.DockerImageReference, err error) {
	upstreamRepo, err := dockerRegistry.Repository(ctx, sourceRepoName)
	if err != nil {
		return distribution.Descriptor{}, nil, err
	}
	namespace, name, err := getNamespaceName(sourceRepoName.Name())
	if err != nil {
		return distribution.Descriptor{}, nil, err
	}

	repo := *destRepo
	repo.namespace = namespace
	repo.name = name
	repo.Repository = upstreamRepo

	// ask pullthrough blob store to set source image reference if the blob is found in remote repository
	var ref imageapi.DockerImageReference
	ctx = WithPullthroughSourceImageReference(ctx, &ref)

	desc, err = repo.Blobs(ctx).Stat(ctx, dgst)
	if err == nil && len(ref.Registry) != 0 {
		pullthroughSourceImageReference = &ref
	}
	return
}

// tagPullthroughSourceImageInTargetRepository creates a tag in a destination image stream of cross-repo mount
// referencing a remote image that contains the blob. With the reference present in the target stream, the
// pullthrough will allow to serve the blob from the image stream without storing it locally.
func tagPullthroughSourceImageInTargetRepository(ctx context.Context, ref *imageapi.DockerImageReference, destRepo *repository, dgst digest.Digest) error {
	if len(ref.ID) == 0 {
		return fmt.Errorf("cannot tag image lacking ID as a pullthrough source (%s)", ref.Exact())
	}

	tag := fmt.Sprintf("_pullthrough_dep_%s", dgst.Hex()[0:6])

	is, err := destRepo.getImageStream()
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}

		// create image stream
		stream := imageapi.ImageStream{
			ObjectMeta: kapi.ObjectMeta{
				Name: destRepo.name,
			},
		}
		context.GetLogger(ctx).Infof("creating image stream to hold pullthrough source image %q for blob %q", ref.Exact(), dgst.String())
		is, err = destRepo.registryOSClient.ImageStreams(destRepo.namespace).Create(&stream)
		if kerrors.IsAlreadyExists(err) {
			is, err = destRepo.getImageStream()
			if err != nil {
				return err
			}
		}
	}

	_, err = imageapi.ResolveImageID(is, ref.ID)
	if err == nil {
		context.GetLogger(ctx).Debugf("source image %s is already rererenced in image stream", ref.ID)
		return err
	}

	// TODO: there's a danger of creating several similar tags for different blobs during a single image push
	context.GetLogger(ctx).Infof("creating istag %s:%s referencing image %q", destRepo.Named().Name(), tag, ref.ID)
	return destRepo.Tags(ctx).Tag(ctx, tag, distribution.Descriptor{Digest: digest.Digest(ref.ID)})
}
