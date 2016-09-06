package server

import (
	"fmt"
	"sort"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/middleware/registry"
	"github.com/docker/distribution/registry/storage"

	kerrors "k8s.io/kubernetes/pkg/api/errors"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ByGeneration allows for sorting tag events from latest to oldest.
type ByGeneration []*imageapi.TagEvent

func (b ByGeneration) Less(i, j int) bool { return b[i].Generation > b[j].Generation }
func (b ByGeneration) Len() int           { return len(b) }
func (b ByGeneration) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

func init() {
	middleware.RegisterOptions(storage.BlobDescriptorServiceFactory(&blobDescriptorServiceFactory{}))
}

// blobDescriptorServiceFactory needs to be able to work with blobs
// directly without using links. This allows us to ignore the distribution
// of blobs between repositories.
type blobDescriptorServiceFactory struct{}

func (bf *blobDescriptorServiceFactory) BlobAccessController(svc distribution.BlobDescriptorService) distribution.BlobDescriptorService {
	return &blobDescriptorService{svc}
}

type blobDescriptorService struct {
	distribution.BlobDescriptorService
}

// Stat returns a a blob descriptor if the given blob is either linked in repository or is referenced in
// corresponding image stream. This method is invoked from inside of upstream's linkedBlobStore. It expects
// a proper repository object to be set on given context by upper openshift middleware wrappers.
func (bs *blobDescriptorService) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	repo, found := RepositoryFrom(ctx)
	if !found || repo == nil {
		err := fmt.Errorf("failed to retrieve repository from context")
		context.GetLogger(ctx).Error(err)
		return distribution.Descriptor{}, err
	}

	// if there is a repo layer link, return its descriptor
	desc, err := bs.BlobDescriptorService.Stat(ctx, dgst)
	if err == nil {
		// and remember the association
		repo.cachedLayers.RememberDigest(dgst, repo.blobrepositorycachettl, imageapi.DockerImageReference{
			Namespace: repo.namespace,
			Name:      repo.name,
		}.Exact())
		return desc, nil
	}

	context.GetLogger(ctx).Debugf("could not stat layer link %q in repository %q: %v", dgst.String(), repo.Named().Name(), err)

	// verify the blob is stored locally
	desc, err = dockerRegistry.BlobStatter().Stat(ctx, dgst)
	if err != nil {
		return desc, err
	}

	// ensure it's referenced inside of corresponding image stream
	if imageStreamHasBlob(repo, dgst) {
		return desc, nil
	}

	return distribution.Descriptor{}, distribution.ErrBlobUnknown
}

func (bs *blobDescriptorService) Clear(ctx context.Context, dgst digest.Digest) error {
	repo, found := RepositoryFrom(ctx)
	if !found || repo == nil {
		err := fmt.Errorf("failed to retrieve repository from context")
		context.GetLogger(ctx).Error(err)
		return err
	}

	repo.cachedLayers.ForgetDigest(dgst, imageapi.DockerImageReference{
		Namespace: repo.namespace,
		Name:      repo.name,
	}.Exact())
	return bs.BlobDescriptorService.Clear(ctx, dgst)
}

// imageStreamHasBlob returns true if the given blob digest is referenced in image stream corresponding to
// given repository. If not found locally, image stream's images will be iterated and fetched from newest to
// oldest until found. Each processed image will update local cache of blobs.
func imageStreamHasBlob(r *repository, dgst digest.Digest) bool {
	repoCacheName := imageapi.DockerImageReference{Namespace: r.namespace, Name: r.name}.Exact()
	if r.cachedLayers.RepositoryHasBlob(repoCacheName, dgst) {
		context.GetLogger(r.ctx).Debugf("found cached blob %q in repository %s", dgst.String(), r.Named().Name())
		return true
	}

	context.GetLogger(r.ctx).Debugf("verifying presence of blob %q in image stream %s/%s", dgst.String(), r.namespace, r.name)
	started := time.Now()
	logFound := func(found bool) bool {
		elapsed := time.Now().Sub(started)
		if found {
			context.GetLogger(r.ctx).Debugf("verified presence of blob %q in image stream %s/%s after %s", dgst.String(), r.namespace, r.name, elapsed.String())
		} else {
			context.GetLogger(r.ctx).Debugf("detected absence of blob %q in image stream %s/%s after %s", dgst.String(), r.namespace, r.name, elapsed.String())
		}
		return found
	}

	// verify directly with etcd
	is, err := r.getImageStream()
	if err != nil {
		context.GetLogger(r.ctx).Errorf("failed to get image stream: %v", err)
		return logFound(false)
	}

	tagEvents := []*imageapi.TagEvent{}
	event2Name := make(map[*imageapi.TagEvent]string)
	for name, eventList := range is.Status.Tags {
		for i := range eventList.Items {
			event := &eventList.Items[i]
			tagEvents = append(tagEvents, event)
			event2Name[event] = name
		}
	}
	// search from youngest to oldest
	sort.Sort(ByGeneration(tagEvents))

	processedImages := map[string]struct{}{}

	for _, tagEvent := range tagEvents {
		if _, processed := processedImages[tagEvent.Image]; processed {
			continue
		}
		if imageHasBlob(r, repoCacheName, tagEvent.Image, dgst.String(), !r.pullthrough) {
			tagName := event2Name[tagEvent]
			context.GetLogger(r.ctx).Debugf("blob found under istag %s/%s:%s in image %s", r.namespace, r.name, tagName, tagEvent.Image)
			return logFound(true)
		}
		processedImages[tagEvent.Image] = struct{}{}
	}

	context.GetLogger(r.ctx).Warnf("blob %q exists locally but is not referenced in repository %s/%s", dgst.String(), r.namespace, r.name)

	return logFound(false)
}

// imageHasBlob returns true if the image identified by imageName refers to the given blob. The image is
// fetched. If requireManaged is true and the image is not managed (it refers to remote registry), the image
// will not be processed. Fetched image will update local cache of blobs -> repositories with (blobDigest,
// cacheName) pairs.
func imageHasBlob(
	r *repository,
	cacheName,
	imageName,
	blobDigest string,
	requireManaged bool,
) bool {
	context.GetLogger(r.ctx).Debugf("getting image %s", imageName)
	image, err := r.getImage(digest.Digest(imageName))
	if err != nil {
		if kerrors.IsNotFound(err) {
			context.GetLogger(r.ctx).Debugf("image %q not found: imageName")
		} else {
			context.GetLogger(r.ctx).Errorf("failed to get image: %v", err)
		}
		return false
	}

	// in case of pullthrough disabled, client won't be able to download a blob belonging to not managed image
	// (image stored in external registry), thus don't consider them as candidates
	if managed := image.Annotations[imageapi.ManagedByOpenShiftAnnotation]; requireManaged && managed != "true" {
		context.GetLogger(r.ctx).Debugf("skipping not managed image")
		return false
	}

	if len(image.DockerImageLayers) == 0 {
		if len(image.DockerImageManifestMediaType) > 0 {
			// If the media type is set, we can safely assume that the best effort to fill the image layers
			// has already been done. There are none.
			return false
		}
		err = imageapi.ImageWithMetadata(image)
		if err != nil {
			context.GetLogger(r.ctx).Errorf("failed to get metadata for image %s: %v", imageName, err)
			return false
		}
	}

	for _, layer := range image.DockerImageLayers {
		if layer.Name == blobDigest {
			// remember all the layers of matching image
			r.rememberLayersOfImage(image, cacheName)
			return true
		}
	}

	// only manifest V2 schema2 has docker image config filled where dockerImage.Metadata.id is its digest
	if len(image.DockerImageConfig) > 0 && image.DockerImageMetadata.ID == blobDigest {
		// remember manifest config reference of schema 2 as well
		r.rememberLayersOfImage(image, cacheName)
		return true
	}

	return false
}
