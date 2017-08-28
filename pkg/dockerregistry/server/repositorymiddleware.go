package server

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/api/errcode"
	repomw "github.com/docker/distribution/registry/middleware/repository"
	registrystorage "github.com/docker/distribution/registry/storage"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/dockerregistry/server/audit"
	"github.com/openshift/origin/pkg/dockerregistry/server/client"
	"github.com/openshift/origin/pkg/dockerregistry/server/metrics"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

const (
	// Environment variables

	// DockerRegistryURLEnvVar is a mandatory environment variable name specifying url of internal docker
	// registry. All references to pushed images will be prefixed with its value.
	// DEPRECATED: Use the OPENSHIFT_DEFAULT_REGISTRY instead.
	DockerRegistryURLEnvVar = "DOCKER_REGISTRY_URL"

	// OpenShiftDefaultRegistry overrides the DockerRegistryURLEnvVar as in OpenShift the
	// default registry URL is controller by this environment variable.
	OpenShiftDefaultRegistry = "OPENSHIFT_DEFAULT_REGISTRY"

	// EnforceQuotaEnvVar is a boolean environment variable that allows to turn quota enforcement on or off.
	// By default, quota enforcement is off. It overrides openshift middleware configuration option.
	// Recognized values are "true" and "false".
	EnforceQuotaEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_ENFORCEQUOTA"

	// ProjectCacheTTLEnvVar is an environment variable specifying an eviction timeout for project quota
	// objects. It takes a valid time duration string (e.g. "2m"). If empty, you get the default timeout. If
	// zero (e.g. "0m"), caching is disabled.
	ProjectCacheTTLEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_PROJECTCACHETTL"

	// AcceptSchema2EnvVar is a boolean environment variable that allows to accept manifest schema v2
	// on manifest put requests.
	AcceptSchema2EnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_ACCEPTSCHEMA2"

	// BlobRepositoryCacheTTLEnvVar  is an environment variable specifying an eviction timeout for <blob
	// belongs to repository> entries. The higher the value, the faster queries but also a higher risk of
	// leaking a blob that is no longer tagged in given repository.
	BlobRepositoryCacheTTLEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_BLOBREPOSITORYCACHETTL"

	// Pullthrough is a boolean environment variable that controls whether pullthrough is enabled.
	PullthroughEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_PULLTHROUGH"

	// MirrorPullthrough is a boolean environment variable that controls mirroring of blobs on pullthrough.
	MirrorPullthroughEnvVar = "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_MIRRORPULLTHROUGH"

	// Default values

	defaultDigestToRepositoryCacheSize = 2048
	defaultBlobRepositoryCacheTTL      = time.Minute * 10
)

var (
	// cachedLayers is a shared cache of blob digests to repositories that have previously been identified as
	// containing that blob. Thread safe and reused by all middleware layers. It contains two kinds of
	// associations:
	//  1. <blobdigest> <-> <registry>/<namespace>/<name>
	//  2. <blobdigest> <-> <namespace>/<name>
	// The first associates a blob with a remote repository. Such an entry is set and used by pullthrough
	// middleware. The second associates a blob with a local repository. Such a blob is expected to reside on
	// local storage. It's set and used by blobDescriptorService middleware.
	cachedLayers digestToRepositoryCache
	// secureTransport is the transport pool used for pullthrough to remote registries marked as
	// secure.
	secureTransport http.RoundTripper
	// insecureTransport is the transport pool that does not verify remote TLS certificates for use
	// during pullthrough against registries marked as insecure.
	insecureTransport http.RoundTripper
	// quotaEnforcing contains shared caches of quota objects keyed by project name. Will be initialized
	// only if the quota is enforced. See EnforceQuotaEnvVar.
	quotaEnforcing *quotaEnforcingConfig
)

func init() {
	cache, err := newDigestToRepositoryCache(defaultDigestToRepositoryCacheSize)
	if err != nil {
		panic(err)
	}
	cachedLayers = cache

	// load the client when the middleware is initialized, which allows test code to change
	// the registry client before starting a registry.
	repomw.Register("openshift",
		func(ctx context.Context, repo distribution.Repository, options map[string]interface{}) (distribution.Repository, error) {
			if dockerRegistry == nil {
				panic(fmt.Sprintf("Configuration error: OpenShift registry middleware not activated"))
			}

			if dockerStorageDriver == nil {
				panic(fmt.Sprintf("Configuration error: OpenShift storage driver middleware not activated"))
			}

			registryOSClient, errClient := RegistryClientFrom(ctx).Client()
			if errClient != nil {
				return nil, errClient
			}
			if quotaEnforcing == nil {
				quotaEnforcing = newQuotaEnforcingConfig(ctx, os.Getenv(EnforceQuotaEnvVar), os.Getenv(ProjectCacheTTLEnvVar), options)
			}

			return newRepositoryWithClient(ctx, registryOSClient, repo, options)
		},
	)

	secureTransport = http.DefaultTransport
	insecureTransport, err = restclient.TransportFor(&restclient.Config{TLSClientConfig: restclient.TLSClientConfig{Insecure: true}})
	if err != nil {
		panic(fmt.Sprintf("Unable to configure a default transport for importing insecure images: %v", err))
	}
}

// repository wraps a distribution.Repository and allows manifests to be served from the OpenShift image
// API.
type repository struct {
	distribution.Repository

	ctx              context.Context
	registryOSClient client.Interface
	registryAddr     string
	namespace        string
	name             string
	enabledMetrics   bool

	// if true, the repository will check remote references in the image stream to support pulling "through"
	// from a remote repository
	pullthrough bool
	// mirrorPullthrough will mirror remote blobs into the local repository if set
	mirrorPullthrough bool
	// acceptschema2 allows to refuse the manifest schema version 2
	acceptschema2 bool
	// blobrepositorycachettl is an eviction timeout for <blob belongs to repository> entries of cachedLayers
	blobrepositorycachettl time.Duration
	// cachedImages contains images cached for the lifetime of the request being handled.
	cachedImages map[digest.Digest]*imageapiv1.Image
	// cachedImageStream stays cached for the entire time of handling signle repository-scoped request.
	imageStreamGetter *cachedImageStreamGetter
	// cachedLayers remembers a mapping of layer digest to repositories recently seen with that image to avoid
	// having to check every potential upstream repository when a blob request is made. The cache is useful only
	// when session affinity is on for the registry, but in practice the first pull will fill the cache.
	cachedLayers digestToRepositoryCache
	// remoteBlobGetter is used to fetch blobs from remote registries if pullthrough is enabled.
	remoteBlobGetter BlobGetterService
}

// newRepositoryWithClient returns a new repository middleware.
func newRepositoryWithClient(
	ctx context.Context,
	registryOSClient client.Interface,
	repo distribution.Repository,
	options map[string]interface{},
) (distribution.Repository, error) {
	registryConfig := ConfigurationFrom(ctx)
	// TODO: Deprecate this environment variable.
	registryAddr := os.Getenv(DockerRegistryURLEnvVar)
	if len(registryAddr) == 0 {
		registryAddr = os.Getenv(OpenShiftDefaultRegistry)
	} else {
		context.GetLogger(ctx).Infof("DEPRECATED: %q is deprecated, use the %q instead", DockerRegistryURLEnvVar, OpenShiftDefaultRegistry)
	}
	// TODO: This is a fallback to assuming there is a service named 'docker-registry'. This
	// might change in the future and we should make this configurable.
	if len(registryAddr) == 0 {
		if len(os.Getenv("DOCKER_REGISTRY_SERVICE_HOST")) > 0 && len(os.Getenv("DOCKER_REGISTRY_SERVICE_PORT")) > 0 {
			registryAddr = os.Getenv("DOCKER_REGISTRY_SERVICE_HOST") + ":" + os.Getenv("DOCKER_REGISTRY_SERVICE_PORT")
		} else {
			return nil, fmt.Errorf("%s variable must be set when running outside of Kubernetes cluster", DockerRegistryURLEnvVar)
		}
	}
	context.GetLogger(ctx).Infof("Using %q as Docker Registry URL", registryAddr)

	acceptschema2, err := getBoolOption(AcceptSchema2EnvVar, "acceptschema2", true, options)
	if err != nil {
		context.GetLogger(ctx).Error(err)
	}
	blobrepositorycachettl, err := getDurationOption(BlobRepositoryCacheTTLEnvVar, "blobrepositorycachettl", defaultBlobRepositoryCacheTTL, options)
	if err != nil {
		context.GetLogger(ctx).Error(err)
	}
	pullthrough, err := getBoolOption(PullthroughEnvVar, "pullthrough", true, options)
	if err != nil {
		context.GetLogger(ctx).Error(err)
	}
	mirrorPullthrough, err := getBoolOption(MirrorPullthroughEnvVar, "mirrorpullthrough", true, options)
	if err != nil {
		context.GetLogger(ctx).Error(err)
	}

	nameParts := strings.SplitN(repo.Named().Name(), "/", 2)
	if len(nameParts) != 2 {
		return nil, fmt.Errorf("invalid repository name %q: it must be of the format <project>/<name>", repo.Named().Name())
	}
	namespace, name := nameParts[0], nameParts[1]

	imageStreamGetter := &cachedImageStreamGetter{
		ctx:          ctx,
		namespace:    namespace,
		name:         name,
		isNamespacer: registryOSClient,
	}

	r := &repository{
		Repository: repo,

		ctx:                    ctx,
		registryOSClient:       registryOSClient,
		registryAddr:           registryAddr,
		namespace:              nameParts[0],
		name:                   nameParts[1],
		acceptschema2:          acceptschema2,
		blobrepositorycachettl: blobrepositorycachettl,
		pullthrough:            pullthrough,
		mirrorPullthrough:      mirrorPullthrough,
		imageStreamGetter:      imageStreamGetter,
		cachedImages:           make(map[digest.Digest]*imageapiv1.Image),
		cachedLayers:           cachedLayers,
		enabledMetrics:         registryConfig.Metrics.Enabled,
	}

	if pullthrough {
		r.remoteBlobGetter = NewBlobGetterService(
			r.namespace,
			r.name,
			blobrepositorycachettl,
			imageStreamGetter.get,
			registryOSClient,
			cachedLayers)
	}

	return r, nil
}

// Manifests returns r, which implements distribution.ManifestService.
func (r *repository) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {
	// we do a verification of our own
	// TODO: let upstream do the verification once they pass correct context object to their manifest handler
	opts := append(options, registrystorage.SkipLayerVerification())
	ms, err := r.Repository.Manifests(withRepository(ctx, r), opts...)
	if err != nil {
		return nil, err
	}

	ms = &manifestService{
		ctx:           withRepository(ctx, r),
		repo:          r,
		manifests:     ms,
		acceptschema2: r.acceptschema2,
	}

	if r.pullthrough {
		ms = &pullthroughManifestService{
			ManifestService: ms,
			repo:            r,
		}
	}

	ms = &errorManifestService{
		manifests: ms,
		repo:      r,
	}

	if audit.LoggerExists(ctx) {
		ms = audit.NewManifestService(ctx, ms)
	}

	if r.enabledMetrics {
		ms = &metrics.ManifestService{
			Manifests: ms,
			Reponame:  r.Named().Name(),
		}
	}

	return ms, nil
}

// Blobs returns a blob store which can delegate to remote repositories.
func (r *repository) Blobs(ctx context.Context) distribution.BlobStore {
	bs := r.Repository.Blobs(ctx)

	if quotaEnforcing.enforcementEnabled {
		bs = &quotaRestrictedBlobStore{
			BlobStore: bs,

			repo: r,
		}
	}

	if r.pullthrough {
		bs = &pullthroughBlobStore{
			BlobStore: bs,

			repo:   r,
			mirror: r.mirrorPullthrough,
		}
	}

	bs = &errorBlobStore{
		store: bs,
		repo:  r,
	}

	if audit.LoggerExists(ctx) {
		bs = audit.NewBlobStore(ctx, bs)
	}

	if r.enabledMetrics {
		bs = &metrics.BlobStore{
			Store:    bs,
			Reponame: r.Named().Name(),
		}
	}

	return bs
}

// Tags returns a reference to this repository tag service.
func (r *repository) Tags(ctx context.Context) distribution.TagService {
	var ts distribution.TagService

	ts = &tagService{
		TagService: r.Repository.Tags(ctx),
		repo:       r,
	}

	ts = &errorTagService{
		tags: ts,
		repo: r,
	}

	if audit.LoggerExists(ctx) {
		ts = audit.NewTagService(ctx, ts)
	}

	if r.enabledMetrics {
		ts = &metrics.TagService{
			Tags:     ts,
			Reponame: r.Named().Name(),
		}
	}

	return ts
}

// createImageStream creates a new image stream corresponding to r and caches it.
func (r *repository) createImageStream(ctx context.Context) (*imageapiv1.ImageStream, error) {
	stream := imageapiv1.ImageStream{}
	stream.Name = r.name

	uclient, ok := userClientFrom(ctx)
	if !ok {
		errmsg := "error creating user client to auto provision image stream: user client to master API unavailable"
		context.GetLogger(ctx).Errorf(errmsg)
		return nil, errcode.ErrorCodeUnknown.WithDetail(errmsg)
	}

	is, err := uclient.ImageStreams(r.namespace).Create(&stream)
	switch {
	case kerrors.IsAlreadyExists(err), kerrors.IsConflict(err):
		context.GetLogger(ctx).Infof("conflict while creating ImageStream: %v", err)
		return r.imageStreamGetter.get()
	case kerrors.IsForbidden(err), kerrors.IsUnauthorized(err), quotautil.IsErrorQuotaExceeded(err):
		context.GetLogger(ctx).Errorf("denied creating ImageStream: %v", err)
		return nil, errcode.ErrorCodeDenied.WithDetail(err)
	case err != nil:
		context.GetLogger(ctx).Errorf("error auto provisioning ImageStream: %s", err)
		return nil, errcode.ErrorCodeUnknown.WithDetail(err)
	}

	r.imageStreamGetter.cacheImageStream(is)
	return is, nil
}

// getImage retrieves the Image with digest `dgst`. No authorization check is done.
func (r *repository) getImage(dgst digest.Digest) (*imageapiv1.Image, error) {
	if image, exists := r.cachedImages[dgst]; exists {
		context.GetLogger(r.ctx).Infof("(*repository).getImage: returning cached copy of %s", image.Name)
		return image, nil
	}

	image, err := r.registryOSClient.Images().Get(dgst.String(), metav1.GetOptions{})
	if err != nil {
		context.GetLogger(r.ctx).Errorf("failed to get image: %v", err)
		return nil, wrapKStatusErrorOnGetImage(r.name, dgst, err)
	}

	context.GetLogger(r.ctx).Infof("(*repository).getImage: got image %s", image.Name)
	if err := imageapiv1.ImageWithMetadata(image); err != nil {
		return nil, err
	}
	r.cachedImages[dgst] = image
	return image, nil
}

// getStoredImageOfImageStream retrieves the Image with digest `dgst` and
// ensures that the image belongs to the ImageStream associated with r. It
// uses two queries to master API:
//
//  1st to get a corresponding image stream
//  2nd to get the image
//
// This allows us to cache the image stream for later use.
//
// If you need the image object to be modified according to image stream tag,
// please use getImageOfImageStream.
func (r *repository) getStoredImageOfImageStream(dgst digest.Digest) (*imageapiv1.Image, *imageapiv1.TagEvent, *imageapiv1.ImageStream, error) {
	stream, err := r.imageStreamGetter.get()
	if err != nil {
		context.GetLogger(r.ctx).Errorf("failed to get ImageStream: %v", err)
		return nil, nil, nil, wrapKStatusErrorOnGetImage(r.name, dgst, err)
	}

	tagEvent, err := imageapiv1.ResolveImageID(stream, dgst.String())
	if err != nil {
		context.GetLogger(r.ctx).Errorf("failed to resolve image %s in ImageStream %s/%s: %v", dgst.String(), r.namespace, r.name, err)
		return nil, nil, nil, wrapKStatusErrorOnGetImage(r.name, dgst, err)
	}

	image, err := r.getImage(dgst)
	if err != nil {
		return nil, nil, nil, wrapKStatusErrorOnGetImage(r.name, dgst, err)
	}

	return image, tagEvent, stream, nil
}

// getImageOfImageStream retrieves the Image with digest `dgst` for
// the ImageStream associated with r. The image's field DockerImageReference
// is modified on the fly to pretend that we've got the image from the source
// from which the image was tagged.to match tag's DockerImageReference.
//
// NOTE: due to on the fly modification, the returned image object should
// not be sent to the master API. If you need unmodified version of the
// image object, please use getStoredImageOfImageStream.
func (r *repository) getImageOfImageStream(dgst digest.Digest) (*imageapiv1.Image, *imageapiv1.ImageStream, error) {
	image, tagEvent, stream, err := r.getStoredImageOfImageStream(dgst)
	if err != nil {
		return nil, nil, err
	}

	image.DockerImageReference = tagEvent.DockerImageReference

	return image, stream, nil
}

// updateImage modifies the Image.
func (r *repository) updateImage(image *imageapiv1.Image) (*imageapiv1.Image, error) {
	return r.registryOSClient.Images().Update(image)
}

// rememberLayersOfImage caches the layer digests of given image
func (r *repository) rememberLayersOfImage(image *imageapiv1.Image, cacheName string) {
	if len(image.DockerImageLayers) == 0 && len(image.DockerImageManifestMediaType) > 0 && len(image.DockerImageConfig) == 0 {
		// image has no layers
		return
	}

	if len(image.DockerImageLayers) > 0 {
		for _, layer := range image.DockerImageLayers {
			r.cachedLayers.RememberDigest(digest.Digest(layer.Name), r.blobrepositorycachettl, cacheName)
		}
		meta, ok := image.DockerImageMetadata.Object.(*imageapi.DockerImage)
		if !ok {
			context.GetLogger(r.ctx).Errorf("image does not have metadata %s", image.Name)
			return
		}
		// remember reference to manifest config as well for schema 2
		if image.DockerImageManifestMediaType == schema2.MediaTypeManifest && len(meta.ID) > 0 {
			r.cachedLayers.RememberDigest(digest.Digest(meta.ID), r.blobrepositorycachettl, cacheName)
		}
		return
	}
	mh, err := NewManifestHandlerFromImage(r, image)
	if err != nil {
		context.GetLogger(r.ctx).Errorf("cannot remember layers of image %q: %v", image.Name, err)
		return
	}
	dgst, err := mh.Digest()
	if err != nil {
		context.GetLogger(r.ctx).Errorf("cannot get manifest digest of image %q: %v", image.Name, err)
		return
	}

	r.rememberLayersOfManifest(dgst, mh.Manifest(), cacheName)
}

// rememberLayersOfManifest caches the layer digests of given manifest
func (r *repository) rememberLayersOfManifest(manifestDigest digest.Digest, manifest distribution.Manifest, cacheName string) {
	r.cachedLayers.RememberDigest(manifestDigest, r.blobrepositorycachettl, cacheName)

	// remember the layers in the cache as an optimization to avoid searching all remote repositories
	for _, layer := range manifest.References() {
		r.cachedLayers.RememberDigest(layer.Digest, r.blobrepositorycachettl, cacheName)
	}
}

// manifestFromImageWithCachedLayers loads the image and then caches any located layers
func (r *repository) manifestFromImageWithCachedLayers(image *imageapiv1.Image, cacheName string) (manifest distribution.Manifest, err error) {
	mh, err := NewManifestHandlerFromImage(r, image)
	if err != nil {
		return
	}
	dgst, err := mh.Digest()
	if err != nil {
		context.GetLogger(r.ctx).Errorf("cannot get payload from manifest handler: %v", err)
		return
	}
	manifest = mh.Manifest()

	r.rememberLayersOfManifest(dgst, manifest, cacheName)
	return
}

func (r *repository) checkPendingErrors(ctx context.Context) error {
	return checkPendingErrors(ctx, context.GetLogger(r.ctx), r.namespace, r.name)
}

func checkPendingErrors(ctx context.Context, logger context.Logger, namespace, name string) error {
	if !authPerformed(ctx) {
		return fmt.Errorf("openshift.auth.completed missing from context")
	}

	deferredErrors, haveDeferredErrors := deferredErrorsFrom(ctx)
	if !haveDeferredErrors {
		return nil
	}

	repoErr, haveRepoErr := deferredErrors.Get(namespace, name)
	if !haveRepoErr {
		return nil
	}

	logger.Debugf("Origin auth: found deferred error for %s/%s: %v", namespace, name, repoErr)
	return repoErr
}
