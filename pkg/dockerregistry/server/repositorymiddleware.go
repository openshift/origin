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
	regapi "github.com/docker/distribution/registry/api/v2"
	repomw "github.com/docker/distribution/registry/middleware/repository"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/importer"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

const (
	// Environment variables

	// DockerRegistryURLEnvVar is a mandatory environment variable name specifying url of internal docker
	// registry. All references to pushed images will be prefixed with its value.
	DockerRegistryURLEnvVar = "DOCKER_REGISTRY_URL"

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
	// DefaultRegistryClient before starting a registry.
	repomw.Register("openshift",
		func(ctx context.Context, repo distribution.Repository, options map[string]interface{}) (distribution.Repository, error) {
			if dockerRegistry == nil {
				panic(fmt.Sprintf("Configuration error: OpenShift registry middleware not activated"))
			}

			if dockerStorageDriver == nil {
				panic(fmt.Sprintf("Configuration error: OpenShift storage driver middleware not activated"))
			}

			registryOSClient, kClient, errClients := DefaultRegistryClient.Clients()
			if errClients != nil {
				return nil, errClients
			}
			if quotaEnforcing == nil {
				quotaEnforcing = newQuotaEnforcingConfig(ctx, os.Getenv(EnforceQuotaEnvVar), os.Getenv(ProjectCacheTTLEnvVar), options)
			}

			return newRepositoryWithClient(registryOSClient, kClient, kClient, ctx, repo, options)
		},
	)

	secureTransport = http.DefaultTransport
	insecureTransport, err = restclient.TransportFor(&restclient.Config{Insecure: true})
	if err != nil {
		panic(fmt.Sprintf("Unable to configure a default transport for importing insecure images: %v", err))
	}
}

// repository wraps a distribution.Repository and allows manifests to be served from the OpenShift image
// API.
type repository struct {
	distribution.Repository

	ctx              context.Context
	quotaClient      kclient.ResourceQuotasNamespacer
	limitClient      kclient.LimitRangesNamespacer
	registryOSClient client.Interface
	registryAddr     string
	namespace        string
	name             string

	// if true, the repository will check remote references in the image stream to support pulling "through"
	// from a remote repository
	pullthrough bool
	// acceptschema2 allows to refuse the manifest schema version 2
	acceptschema2 bool
	// blobrepositorycachettl is an eviction timeout for <blob belongs to repository> entries of cachedLayers
	blobrepositorycachettl time.Duration
	// cachedLayers remembers a mapping of layer digest to repositories recently seen with that image to avoid
	// having to check every potential upstream repository when a blob request is made. The cache is useful only
	// when session affinity is on for the registry, but in practice the first pull will fill the cache.
	cachedLayers digestToRepositoryCache
}

var _ distribution.ManifestService = &repository{}

// newRepositoryWithClient returns a new repository middleware.
func newRepositoryWithClient(
	registryOSClient client.Interface,
	quotaClient kclient.ResourceQuotasNamespacer,
	limitClient kclient.LimitRangesNamespacer,
	ctx context.Context,
	repo distribution.Repository,
	options map[string]interface{},
) (distribution.Repository, error) {
	registryAddr := os.Getenv(DockerRegistryURLEnvVar)
	if len(registryAddr) == 0 {
		return nil, fmt.Errorf("%s is required", DockerRegistryURLEnvVar)
	}

	acceptschema2, err := getBoolOption(AcceptSchema2EnvVar, "acceptschema2", false, options)
	if err != nil {
		context.GetLogger(ctx).Error(err)
	}
	blobrepositorycachettl, err := getDurationOption(BlobRepositoryCacheTTLEnvVar, "blobrepositorycachettl", defaultBlobRepositoryCacheTTL, options)
	if err != nil {
		context.GetLogger(ctx).Error(err)
	}
	pullthrough, err := getBoolOption("", "pullthrough", false, options)
	if err != nil {
		context.GetLogger(ctx).Error(err)
	}

	nameParts := strings.SplitN(repo.Named().Name(), "/", 2)
	if len(nameParts) != 2 {
		return nil, fmt.Errorf("invalid repository name %q: it must be of the format <project>/<name>", repo.Named().Name())
	}

	return &repository{
		Repository: repo,

		ctx:                    ctx,
		quotaClient:            quotaClient,
		limitClient:            limitClient,
		registryOSClient:       registryOSClient,
		registryAddr:           registryAddr,
		namespace:              nameParts[0],
		name:                   nameParts[1],
		acceptschema2:          acceptschema2,
		blobrepositorycachettl: blobrepositorycachettl,
		pullthrough:            pullthrough,
		cachedLayers:           cachedLayers,
	}, nil
}

// Manifests returns r, which implements distribution.ManifestService.
func (r *repository) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {
	if r.ctx == ctx {
		return r, nil
	}
	repo := repository(*r)
	repo.ctx = ctx
	return &repo, nil
}

// Blobs returns a blob store which can delegate to remote repositories.
func (r *repository) Blobs(ctx context.Context) distribution.BlobStore {
	repo := repository(*r)
	repo.ctx = ctx

	bs := r.Repository.Blobs(ctx)

	if !quotaEnforcing.enforcementDisabled {
		bs = &quotaRestrictedBlobStore{
			BlobStore: bs,

			repo: &repo,
		}
	}

	if r.pullthrough {
		bs = &pullthroughBlobStore{
			BlobStore: bs,

			repo:          &repo,
			digestToStore: make(map[string]distribution.BlobStore),
		}
	}

	bs = &errorBlobStore{
		store: bs,
		repo:  &repo,
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

	return ts
}

// Exists returns true if the manifest specified by dgst exists.
func (r *repository) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	if err := r.checkPendingErrors(ctx); err != nil {
		return false, err
	}

	image, err := r.getImage(dgst)
	if err != nil {
		return false, err
	}
	return image != nil, nil
}

// Get retrieves the manifest with digest `dgst`.
func (r *repository) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	if err := r.checkPendingErrors(ctx); err != nil {
		return nil, err
	}

	if _, err := r.getImageStreamImage(dgst); err != nil {
		context.GetLogger(r.ctx).Errorf("error retrieving ImageStreamImage %s/%s@%s: %v", r.namespace, r.name, dgst.String(), err)
		return nil, err
	}

	image, err := r.getImage(dgst)
	if err != nil {
		context.GetLogger(r.ctx).Errorf("error retrieving image %s: %v", dgst.String(), err)
		return nil, err
	}

	ref := imageapi.DockerImageReference{Namespace: r.namespace, Name: r.name, Registry: r.registryAddr}
	if managed := image.Annotations[imageapi.ManagedByOpenShiftAnnotation]; managed == "true" {
		// Repository without a registry part is refers to repository containing locally managed images.
		// Such an entry is retrieved, checked and set by blobDescriptorService operating only on local blobs.
		ref.Registry = ""
	} else {
		// Repository with a registry points to remote repository. This is used by pullthrough middleware.
		ref = ref.DockerClientDefaults().AsRepository()
	}

	manifest, err := r.manifestFromImageWithCachedLayers(image, ref.Exact())

	return manifest, err
}

// Put creates or updates the named manifest.
func (r *repository) Put(ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption) (digest.Digest, error) {
	if err := r.checkPendingErrors(ctx); err != nil {
		return "", err
	}

	mh, err := NewManifestHandler(r, manifest)
	if err != nil {
		return "", regapi.ErrorCodeManifestInvalid.WithDetail(err)
	}
	mediaType, payload, canonical, err := mh.Payload()
	if err != nil {
		return "", regapi.ErrorCodeManifestInvalid.WithDetail(err)
	}

	if !r.acceptschema2 && mediaType == schema2.MediaTypeManifest {
		err = fmt.Errorf("manifest V2 schema 2 not allowed")
		return "", regapi.ErrorCodeManifestInvalid.WithDetail(err)
	}

	// Calculate digest
	dgst := digest.FromBytes(canonical)

	// Upload to openshift
	ism := imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: r.namespace,
			Name:      r.name,
		},
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: dgst.String(),
				Annotations: map[string]string{
					imageapi.ManagedByOpenShiftAnnotation: "true",
				},
			},
			DockerImageReference:         fmt.Sprintf("%s/%s/%s@%s", r.registryAddr, r.namespace, r.name, dgst.String()),
			DockerImageManifest:          string(payload),
			DockerImageManifestMediaType: mediaType,
		},
	}

	for _, option := range options {
		if opt, ok := option.(distribution.WithTagOption); ok {
			ism.Tag = opt.Tag
			break
		}
	}

	if err = mh.FillImageMetadata(ctx, &ism.Image); err != nil {
		return "", err
	}

	if err = r.registryOSClient.ImageStreamMappings(r.namespace).Create(&ism); err != nil {
		// if the error was that the image stream wasn't found, try to auto provision it
		statusErr, ok := err.(*kerrors.StatusError)
		if !ok {
			context.GetLogger(r.ctx).Errorf("error creating ImageStreamMapping: %s", err)
			return "", err
		}

		if quotautil.IsErrorQuotaExceeded(statusErr) {
			context.GetLogger(r.ctx).Errorf("denied creating ImageStreamMapping: %v", statusErr)
			return "", distribution.ErrAccessDenied
		}

		status := statusErr.ErrStatus
		if status.Code != http.StatusNotFound || (strings.ToLower(status.Details.Kind) != "imagestream" /*pre-1.2*/ && strings.ToLower(status.Details.Kind) != "imagestreams") || status.Details.Name != r.name {
			context.GetLogger(r.ctx).Errorf("error creating ImageStreamMapping: %s", err)
			return "", err
		}

		stream := imageapi.ImageStream{
			ObjectMeta: kapi.ObjectMeta{
				Name: r.name,
			},
		}

		uclient, ok := UserClientFrom(r.ctx)
		if !ok {
			context.GetLogger(r.ctx).Errorf("error creating user client to auto provision image stream: Origin user client unavailable")
			return "", statusErr
		}

		if _, err := uclient.ImageStreams(r.namespace).Create(&stream); err != nil {
			if quotautil.IsErrorQuotaExceeded(err) {
				context.GetLogger(r.ctx).Errorf("denied creating ImageStream: %v", err)
				return "", distribution.ErrAccessDenied
			}
			context.GetLogger(r.ctx).Errorf("error auto provisioning ImageStream: %s", err)
			return "", statusErr
		}

		// try to create the ISM again
		if err := r.registryOSClient.ImageStreamMappings(r.namespace).Create(&ism); err != nil {
			if quotautil.IsErrorQuotaExceeded(err) {
				context.GetLogger(r.ctx).Errorf("denied a creation of ImageStreamMapping: %v", err)
				return "", distribution.ErrAccessDenied
			}
			context.GetLogger(r.ctx).Errorf("error creating ImageStreamMapping: %s", err)
			return "", err
		}
	}

	return dgst, nil
}

// Delete deletes the manifest with digest `dgst`. Note: Image resources
// in OpenShift are deleted via 'oadm prune images'. This function deletes
// the content related to the manifest in the registry's storage (signatures).
func (r *repository) Delete(ctx context.Context, dgst digest.Digest) error {
	if err := r.checkPendingErrors(ctx); err != nil {
		return err
	}

	ms, err := r.Repository.Manifests(r.ctx)
	if err != nil {
		return err
	}
	ctx = WithRepository(ctx, r)
	return ms.Delete(ctx, dgst)
}

// importContext loads secrets for this image stream and returns a context for getting distribution
// clients to remote repositories.
func (r *repository) importContext() importer.RepositoryRetriever {
	secrets, err := r.registryOSClient.ImageStreamSecrets(r.namespace).Secrets(r.name, kapi.ListOptions{})
	if err != nil {
		context.GetLogger(r.ctx).Errorf("error getting secrets for repository %q: %v", r.Named().Name(), err)
		secrets = &kapi.SecretList{}
	}
	credentials := importer.NewCredentialsForSecrets(secrets.Items)
	return importer.NewContext(secureTransport, insecureTransport).WithCredentials(credentials)
}

// getImageStream retrieves the ImageStream for r.
func (r *repository) getImageStream() (*imageapi.ImageStream, error) {
	return r.registryOSClient.ImageStreams(r.namespace).Get(r.name)
}

// getImage retrieves the Image with digest `dgst`.
func (r *repository) getImage(dgst digest.Digest) (*imageapi.Image, error) {
	return r.registryOSClient.Images().Get(dgst.String())
}

// getImageStreamImage retrieves the Image with digest `dgst` for the ImageStream
// associated with r. This ensures the image belongs to the image stream.
func (r *repository) getImageStreamImage(dgst digest.Digest) (*imageapi.ImageStreamImage, error) {
	return r.registryOSClient.ImageStreamImages(r.namespace).Get(r.name, dgst.String())
}

// rememberLayersOfImage caches the layer digests of given image
func (r *repository) rememberLayersOfImage(image *imageapi.Image, cacheName string) {
	if len(image.DockerImageLayers) == 0 && len(image.DockerImageManifestMediaType) > 0 && len(image.DockerImageConfig) == 0 {
		// image has no layers
		return
	}

	if len(image.DockerImageLayers) > 0 {
		for _, layer := range image.DockerImageLayers {
			r.cachedLayers.RememberDigest(digest.Digest(layer.Name), r.blobrepositorycachettl, cacheName)
		}
		// remember reference to manifest config as well for schema 2
		if image.DockerImageManifestMediaType == schema2.MediaTypeManifest && len(image.DockerImageMetadata.ID) > 0 {
			r.cachedLayers.RememberDigest(digest.Digest(image.DockerImageMetadata.ID), r.blobrepositorycachettl, cacheName)
		}
		return
	}

	mh, err := NewManifestHandlerFromImage(r, image)
	if err != nil {
		context.GetLogger(r.ctx).Errorf("cannot remember layers of image %q: %v", image.Name, err)
		return
	}
	r.rememberLayersOfManifest(mh.Manifest(), cacheName)
}

// rememberLayersOfManifest caches the layer digests of given manifest
func (r *repository) rememberLayersOfManifest(manifest distribution.Manifest, cacheName string) {
	// remember the layers in the cache as an optimization to avoid searching all remote repositories
	for _, layer := range manifest.References() {
		r.cachedLayers.RememberDigest(layer.Digest, r.blobrepositorycachettl, cacheName)
	}
}

// manifestFromImageWithCachedLayers loads the image and then caches any located layers
func (r *repository) manifestFromImageWithCachedLayers(image *imageapi.Image, cacheName string) (manifest distribution.Manifest, err error) {
	mh, err := NewManifestHandlerFromImage(r, image)
	if err != nil {
		return
	}

	manifest = mh.Manifest()
	r.rememberLayersOfManifest(manifest, cacheName)
	return
}

func (r *repository) checkPendingErrors(ctx context.Context) error {
	return checkPendingErrors(context.GetLogger(r.ctx), ctx, r.namespace, r.name)
}

func checkPendingErrors(logger context.Logger, ctx context.Context, namespace, name string) error {
	if !AuthPerformed(ctx) {
		return fmt.Errorf("openshift.auth.completed missing from context")
	}

	deferredErrors, haveDeferredErrors := DeferredErrorsFrom(ctx)
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
