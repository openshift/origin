package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	repomw "github.com/docker/distribution/registry/middleware/repository"
	"github.com/docker/libtrust"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/importer"
)

var (
	// cachedLayers is a shared cache of blob digests to remote repositories that have previously
	// been identified as containing that blob. Thread safe and reused by all middleware layers.
	cachedLayers digestToRepositoryCache
	// secureTransport is the transport pool used for pullthrough to remote registries marked as
	// secure.
	secureTransport http.RoundTripper
	// insecureTransport is the transport pool that does not verify remote TLS certificates for use
	// during pullthrough against registries marked as insecure.
	insecureTransport http.RoundTripper
)

func init() {
	cache, err := newDigestToRepositoryCache(1024)
	if err != nil {
		panic(err)
	}
	cachedLayers = cache

	// load the client when the middleware is initialized, which allows test code to change
	// DefaultRegistryClient before starting a registry.
	repomw.Register("openshift",
		func(ctx context.Context, repo distribution.Repository, options map[string]interface{}) (distribution.Repository, error) {
			registryClient, quotaClient, err := DefaultRegistryClient.Clients()
			if err != nil {
				return nil, err
			}
			return newRepositoryWithClient(registryClient, quotaClient, ctx, repo, options)
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

	ctx            context.Context
	quotaClient    kclient.ResourceQuotasNamespacer
	registryClient client.Interface
	registryAddr   string
	namespace      string
	name           string

	// if true, the repository will check remote references in the image stream to support pulling "through"
	// from a remote repository
	pullthrough bool
	// cachedLayers remembers a mapping of layer digest to repositories recently seen with that image to avoid
	// having to check every potential upstream repository when a blob request is made. The cache is useful only
	// when session affinity is on for the registry, but in practice the first pull will fill the cache.
	cachedLayers digestToRepositoryCache
}

var _ distribution.ManifestService = &repository{}

// newRepositoryWithClient returns a new repository middleware.
func newRepositoryWithClient(registryClient client.Interface, quotaClient kclient.ResourceQuotasNamespacer, ctx context.Context, repo distribution.Repository, options map[string]interface{}) (distribution.Repository, error) {
	registryAddr := os.Getenv("DOCKER_REGISTRY_URL")
	if len(registryAddr) == 0 {
		return nil, errors.New("DOCKER_REGISTRY_URL is required")
	}

	pullthrough := false
	if value, ok := options["pullthrough"]; ok {
		if b, ok := value.(bool); ok {
			pullthrough = b
		}
	}

	nameParts := strings.SplitN(repo.Name(), "/", 2)
	if len(nameParts) != 2 {
		return nil, fmt.Errorf("invalid repository name %q: it must be of the format <project>/<name>", repo.Name())
	}

	return &repository{
		Repository: repo,

		ctx:            ctx,
		quotaClient:    quotaClient,
		registryClient: registryClient,
		registryAddr:   registryAddr,
		namespace:      nameParts[0],
		name:           nameParts[1],
		pullthrough:    pullthrough,
		cachedLayers:   cachedLayers,
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

	bs := &quotaRestrictedBlobStore{
		BlobStore: r.Repository.Blobs(ctx),
		repo:      &repo,
	}
	if !r.pullthrough {
		return bs
	}

	return &pullthroughBlobStore{
		BlobStore: bs,

		repo:          &repo,
		digestToStore: make(map[string]distribution.BlobStore),
	}
}

// Tags lists the tags under the named repository.
func (r *repository) Tags() ([]string, error) {
	imageStream, err := r.getImageStream()
	if err != nil {
		return []string{}, nil
	}
	tags := []string{}
	for tag := range imageStream.Status.Tags {
		tags = append(tags, tag)
	}

	return tags, nil
}

// Exists returns true if the manifest specified by dgst exists.
func (r *repository) Exists(dgst digest.Digest) (bool, error) {
	image, err := r.getImage(dgst)
	if err != nil {
		return false, err
	}
	return image != nil, nil
}

// ExistsByTag returns true if the manifest with tag `tag` exists.
func (r *repository) ExistsByTag(tag string) (bool, error) {
	imageStream, err := r.getImageStream()
	if err != nil {
		return false, err
	}
	_, found := imageStream.Status.Tags[tag]
	return found, nil
}

// Get retrieves the manifest with digest `dgst`.
func (r *repository) Get(dgst digest.Digest) (*schema1.SignedManifest, error) {
	if _, err := r.getImageStreamImage(dgst); err != nil {
		context.GetLogger(r.ctx).Errorf("Error retrieving ImageStreamImage %s/%s@%s: %v", r.namespace, r.name, dgst.String(), err)
		return nil, err
	}

	image, err := r.getImage(dgst)
	if err != nil {
		context.GetLogger(r.ctx).Errorf("Error retrieving image %s: %v", dgst.String(), err)
		return nil, err
	}

	ref := imageapi.DockerImageReference{Namespace: r.namespace, Name: r.name, Registry: r.registryAddr}
	return r.manifestFromImageWithCachedLayers(image, ref.DockerClientDefaults().Exact())
}

// Enumerate retrieves digests of manifest revisions in particular repository
func (r *repository) Enumerate() ([]digest.Digest, error) {
	panic("not implemented")
}

// GetByTag retrieves the named manifest with the provided tag
func (r *repository) GetByTag(tag string, options ...distribution.ManifestServiceOption) (*schema1.SignedManifest, error) {
	for _, opt := range options {
		if err := opt(r); err != nil {
			return nil, err
		}
	}

	// find the image mapped to this tag
	imageStreamTag, err := r.getImageStreamTag(tag)
	if err != nil {
		// TODO: typed errors
		context.GetLogger(r.ctx).Errorf("Error getting ImageStreamTag %q: %v", tag, err)
		return nil, err
	}
	image := &imageStreamTag.Image

	ref, referenceErr := imageapi.ParseDockerImageReference(image.DockerImageReference)
	if referenceErr == nil {
		ref.Namespace = r.namespace
		ref.Name = r.name
		ref.Registry = r.registryAddr
	}
	defaultRef := ref.DockerClientDefaults()
	cacheName := defaultRef.AsRepository().Exact()

	// if we have a local manifest, use it
	if len(image.DockerImageManifest) > 0 {
		return r.manifestFromImageWithCachedLayers(image, cacheName)
	}

	dgst, err := digest.ParseDigest(imageStreamTag.Image.Name)
	if err != nil {
		context.GetLogger(r.ctx).Errorf("Error parsing digest %q: %v", imageStreamTag.Image.Name, err)
		return nil, err
	}

	if localImage, err := r.getImage(dgst); err != nil {
		// if the image is managed by OpenShift and we cannot load the image, report an error
		if image.Annotations[imageapi.ManagedByOpenShiftAnnotation] == "true" {
			context.GetLogger(r.ctx).Errorf("Error getting image %q: %v", dgst.String(), err)
			return nil, err
		}
	} else {
		// if we have a local manifest, use it
		if len(localImage.DockerImageManifest) > 0 {
			return r.manifestFromImageWithCachedLayers(localImage, cacheName)
		}
	}

	// allow pullthrough to be disabled
	if !r.pullthrough {
		return nil, distribution.ErrManifestBlobUnknown{Digest: dgst}
	}

	// check the previous error here
	if referenceErr != nil {
		context.GetLogger(r.ctx).Errorf("Error parsing image %q: %v", image.DockerImageReference, referenceErr)
		return nil, referenceErr
	}

	return r.pullthroughGetByTag(image, ref, cacheName, options...)
}

// pullthroughGetByTag attempts to load the given image manifest from the remote server defined by ref, using cacheName to store any cached layers.
func (r *repository) pullthroughGetByTag(image *imageapi.Image, ref imageapi.DockerImageReference, cacheName string, options ...distribution.ManifestServiceOption) (*schema1.SignedManifest, error) {
	defaultRef := ref.DockerClientDefaults()

	retriever := r.importContext()

	repo, err := retriever.Repository(r.ctx, defaultRef.RegistryURL(), defaultRef.RepositoryName(), false)
	if err != nil {
		context.GetLogger(r.ctx).Errorf("Error getting remote repository for image %q: %v", image.DockerImageReference, err)
		return nil, err
	}

	// get a manifest context
	manifests, err := repo.Manifests(r.ctx)
	if err != nil {
		context.GetLogger(r.ctx).Errorf("Error getting manifests for image %q: %v", image.DockerImageReference, err)
		return nil, err
	}

	// fetch this by image
	if len(ref.ID) > 0 {
		dgst, err := digest.ParseDigest(ref.ID)
		if err != nil {
			context.GetLogger(r.ctx).Errorf("Error getting manifests for image %q: %v", image.DockerImageReference, err)
			return nil, err
		}
		manifest, err := manifests.Get(dgst)
		if err != nil {
			context.GetLogger(r.ctx).Errorf("Error getting manifest from remote server for image %q: %v", image.DockerImageReference, err)
			return nil, err
		}
		r.rememberLayers(manifest, cacheName)
		return manifest, nil
	}

	// fetch this by tag
	manifest, err := manifests.GetByTag(ref.Tag, options...)
	if err != nil {
		context.GetLogger(r.ctx).Errorf("Error getting manifest from remote server for image %q: %v", image.DockerImageReference, err)
		return nil, err
	}

	r.rememberLayers(manifest, cacheName)
	return manifest, nil
}

// Put creates or updates the named manifest.
func (r *repository) Put(manifest *schema1.SignedManifest) error {
	// Resolve the payload in the manifest.
	payload, err := manifest.Payload()
	if err != nil {
		return err
	}

	// Calculate digest
	dgst, err := digest.FromBytes(payload)
	if err != nil {
		return err
	}

	// Upload to openshift
	ism := imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: r.namespace,
			Name:      r.name,
		},
		Tag: manifest.Tag,
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: dgst.String(),
				Annotations: map[string]string{
					imageapi.ManagedByOpenShiftAnnotation: "true",
				},
			},
			DockerImageReference: fmt.Sprintf("%s/%s/%s@%s", r.registryAddr, r.namespace, r.name, dgst.String()),
			DockerImageManifest:  string(manifest.Raw),
		},
	}

	if err := r.fillImageWithMetadata(manifest, &ism.Image); err != nil {
		return err
	}

	if err := r.registryClient.ImageStreamMappings(r.namespace).Create(&ism); err != nil {
		// if the error was that the image stream wasn't found, try to auto provision it
		statusErr, ok := err.(*kerrors.StatusError)
		if !ok {
			context.GetLogger(r.ctx).Errorf("Error creating ImageStreamMapping: %s", err)
			return err
		}

		if kerrors.IsForbidden(statusErr) {
			context.GetLogger(r.ctx).Errorf("Denied creating ImageStreamMapping: %v", statusErr)
			return distribution.ErrAccessDenied
		}

		status := statusErr.ErrStatus
		if status.Code != http.StatusNotFound || (strings.ToLower(status.Details.Kind) != "imagestream" /*pre-1.2*/ && strings.ToLower(status.Details.Kind) != "imagestreams") || status.Details.Name != r.name {
			context.GetLogger(r.ctx).Errorf("Error creating ImageStreamMapping: %s", err)
			return err
		}

		stream := imageapi.ImageStream{
			ObjectMeta: kapi.ObjectMeta{
				Name: r.name,
			},
		}

		client, ok := UserClientFrom(r.ctx)
		if !ok {
			context.GetLogger(r.ctx).Errorf("Error creating user client to auto provision image stream: Origin user client unavailable")
			return statusErr
		}

		if _, err := client.ImageStreams(r.namespace).Create(&stream); err != nil {
			context.GetLogger(r.ctx).Errorf("Error auto provisioning image stream: %s", err)
			return statusErr
		}

		// try to create the ISM again
		if err := r.registryClient.ImageStreamMappings(r.namespace).Create(&ism); err != nil {
			context.GetLogger(r.ctx).Errorf("Error creating image stream mapping: %s", err)
			return err
		}
	}

	// Grab each json signature and store them.
	signatures, err := manifest.Signatures()
	if err != nil {
		return err
	}

	for _, signature := range signatures {
		if err := r.Signatures().Put(dgst, signature); err != nil {
			context.GetLogger(r.ctx).Errorf("Error storing signature: %s", err)
			return err
		}
	}

	return nil
}

// fillImageWithMetadata fills a given image with metadata. Also correct layer sizes with blob sizes. Newer
// Docker client versions don't set layer sizes in the manifest at all. Origin master needs correct layer
// sizes for proper image quota support. That's why we need to fill the metadata in the registry.
func (r *repository) fillImageWithMetadata(manifest *schema1.SignedManifest, image *imageapi.Image) error {
	if err := imageapi.ImageWithMetadata(image); err != nil {
		return err
	}

	layerSet := sets.NewString()
	size := int64(0)

	blobs := r.Blobs(r.ctx)
	for i := range image.DockerImageLayers {
		layer := &image.DockerImageLayers[i]
		// DockerImageLayers represents manifest.Manifest.FSLayers in reversed order
		desc, err := blobs.Stat(r.ctx, manifest.Manifest.FSLayers[len(image.DockerImageLayers)-i-1].BlobSum)
		if err != nil {
			context.GetLogger(r.ctx).Errorf("Failed to stat blobs %s of image %s", layer.Name, image.DockerImageReference)
			return err
		}
		layer.Size = desc.Size
		// count empty layer just once (empty layer may actually have non-zero size)
		if !layerSet.Has(layer.Name) {
			size += desc.Size
			layerSet.Insert(layer.Name)
		}
	}

	image.DockerImageMetadata.Size = size
	context.GetLogger(r.ctx).Infof("Total size of image %s with docker ref %s: %d", image.Name, image.DockerImageReference, size)

	return nil
}

// Delete deletes the manifest with digest `dgst`. Note: Image resources
// in OpenShift are deleted via 'oadm prune images'. This function deletes
// the content related to the manifest in the registry's storage (signatures).
func (r *repository) Delete(dgst digest.Digest) error {
	ms, err := r.Repository.Manifests(r.ctx)
	if err != nil {
		return err
	}
	return ms.Delete(dgst)
}

// importContext loads secrets for this image stream and returns a context for getting distribution
// clients to remote repositories.
func (r *repository) importContext() importer.RepositoryRetriever {
	secrets, err := r.registryClient.ImageStreamSecrets(r.namespace).Secrets(r.name, kapi.ListOptions{})
	if err != nil {
		context.GetLogger(r.ctx).Errorf("Error getting secrets for repository %q: %v", r.Name(), err)
		secrets = &kapi.SecretList{}
	}
	credentials := importer.NewCredentialsForSecrets(secrets.Items)
	return importer.NewContext(secureTransport, insecureTransport).WithCredentials(credentials)
}

// getImageStream retrieves the ImageStream for r.
func (r *repository) getImageStream() (*imageapi.ImageStream, error) {
	return r.registryClient.ImageStreams(r.namespace).Get(r.name)
}

// getImage retrieves the Image with digest `dgst`.
func (r *repository) getImage(dgst digest.Digest) (*imageapi.Image, error) {
	return r.registryClient.Images().Get(dgst.String())
}

// getImageStreamTag retrieves the Image with tag `tag` for the ImageStream
// associated with r.
func (r *repository) getImageStreamTag(tag string) (*imageapi.ImageStreamTag, error) {
	return r.registryClient.ImageStreamTags(r.namespace).Get(r.name, tag)
}

// getImageStreamImage retrieves the Image with digest `dgst` for the ImageStream
// associated with r. This ensures the image belongs to the image stream.
func (r *repository) getImageStreamImage(dgst digest.Digest) (*imageapi.ImageStreamImage, error) {
	return r.registryClient.ImageStreamImages(r.namespace).Get(r.name, dgst.String())
}

// rememberLayers caches the provided layers
func (r *repository) rememberLayers(manifest *schema1.SignedManifest, cacheName string) {
	if !r.pullthrough {
		return
	}
	// remember the layers in the cache as an optimization to avoid searching all remote repositories
	for _, layer := range manifest.FSLayers {
		r.cachedLayers.RememberDigest(layer.BlobSum, cacheName)
	}
}

// manifestFromImageWithCachedLayers loads the image and then caches any located layers
func (r *repository) manifestFromImageWithCachedLayers(image *imageapi.Image, cacheName string) (*schema1.SignedManifest, error) {
	manifest, err := r.manifestFromImage(image)
	if err != nil {
		return nil, err
	}
	r.rememberLayers(manifest, cacheName)
	return manifest, nil
}

// manifestFromImage converts an Image to a SignedManifest.
func (r *repository) manifestFromImage(image *imageapi.Image) (*schema1.SignedManifest, error) {
	dgst, err := digest.ParseDigest(image.Name)
	if err != nil {
		return nil, err
	}

	raw := []byte(image.DockerImageManifest)

	// prefer signatures from the manifest
	if _, err := libtrust.ParsePrettySignature(raw, "signatures"); err == nil {
		sm := schema1.SignedManifest{Raw: raw}
		if err := json.Unmarshal(raw, &sm); err == nil {
			return &sm, nil
		}
	}

	// Fetch the signatures for the manifest
	signatures, err := r.Signatures().Get(dgst)
	if err != nil {
		return nil, err
	}

	jsig, err := libtrust.NewJSONSignature(raw, signatures...)
	if err != nil {
		return nil, err
	}

	// Extract the pretty JWS
	raw, err = jsig.PrettySignature("signatures")
	if err != nil {
		return nil, err
	}

	var sm schema1.SignedManifest
	if err := json.Unmarshal(raw, &sm); err != nil {
		return nil, err
	}
	return &sm, err
}
