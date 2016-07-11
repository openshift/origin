package importer

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/golang/glog"
	gocontext "golang.org/x/net/context"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/api/errcode"
	registryclient "github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/dockerpre012"
)

// ErrNotV2Registry is returned when the server does not report itself as a V2 Docker registry
type ErrNotV2Registry struct {
	Registry string
}

func (e *ErrNotV2Registry) Error() string {
	return fmt.Sprintf("endpoint %q does not support v2 API", e.Registry)
}

// NewContext is capable of creating RepositoryRetrievers.
func NewContext(transport, insecureTransport http.RoundTripper) Context {
	return Context{
		Transport:         transport,
		InsecureTransport: insecureTransport,
		Challenges:        auth.NewSimpleChallengeManager(),
	}
}

type Context struct {
	Transport         http.RoundTripper
	InsecureTransport http.RoundTripper
	Challenges        auth.ChallengeManager
}

func (c Context) WithCredentials(credentials auth.CredentialStore) RepositoryRetriever {
	return &repositoryRetriever{
		context:     c,
		credentials: credentials,

		pings:    make(map[url.URL]error),
		redirect: make(map[url.URL]*url.URL),
	}
}

type repositoryRetriever struct {
	context     Context
	credentials auth.CredentialStore

	pings    map[url.URL]error
	redirect map[url.URL]*url.URL
}

func (r *repositoryRetriever) Repository(ctx gocontext.Context, registry *url.URL, repoName string, insecure bool) (distribution.Repository, error) {
	named, err := reference.ParseNamed(repoName)
	if err != nil {
		return nil, err
	}

	t := r.context.Transport
	if insecure && r.context.InsecureTransport != nil {
		t = r.context.InsecureTransport
	}
	src := *registry
	// ping the registry to get challenge headers
	if err, ok := r.pings[src]; ok {
		if err != nil {
			return nil, err
		}
		if redirect, ok := r.redirect[src]; ok {
			src = *redirect
		}
	} else {
		redirect, err := r.ping(src, insecure, t)
		r.pings[src] = err
		if err != nil {
			return nil, err
		}
		if redirect != nil {
			r.redirect[src] = redirect
			src = *redirect
		}
	}

	rt := transport.NewTransport(
		t,
		// TODO: slightly smarter authorizer that retries unauthenticated requests
		// TODO: make multiple attempts if the first credential fails
		auth.NewAuthorizer(
			r.context.Challenges,
			auth.NewTokenHandler(t, r.credentials, repoName, "pull"),
			auth.NewBasicHandler(r.credentials),
		),
	)

	repo, err := registryclient.NewRepository(context.Context(ctx), named, src.String(), rt)
	if err != nil {
		return nil, err
	}
	return NewRetryRepository(repo, 2, 3/2*time.Second), nil
}

func (r *repositoryRetriever) ping(registry url.URL, insecure bool, transport http.RoundTripper) (*url.URL, error) {
	pingClient := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
	target := registry
	target.Path = path.Join(target.Path, "v2") + "/"
	req, err := http.NewRequest("GET", target.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := pingClient.Do(req)
	if err != nil {
		if insecure && registry.Scheme == "https" {
			glog.V(5).Infof("Falling back to an HTTP check for an insecure registry %s: %v", registry, err)
			registry.Scheme = "http"
			_, nErr := r.ping(registry, true, transport)
			if nErr != nil {
				return nil, nErr
			}
			return &registry, nil
		}
		return nil, err
	}
	defer resp.Body.Close()

	versions := auth.APIVersions(resp, "Docker-Distribution-API-Version")
	if len(versions) == 0 {
		glog.V(5).Infof("Registry responded to v2 Docker endpoint, but has no header for Docker Distribution %s: %d, %#v", req.URL, resp.StatusCode, resp.Header)
		return nil, &ErrNotV2Registry{Registry: registry.String()}
	}

	r.context.Challenges.AddResponse(resp)

	return nil, nil
}

func schema1ToImage(manifest *schema1.SignedManifest, d digest.Digest) (*api.Image, error) {
	if len(manifest.History) == 0 {
		return nil, fmt.Errorf("image has no v1Compatibility history and cannot be used")
	}
	dockerImage, err := unmarshalDockerImage([]byte(manifest.History[0].V1Compatibility))
	if err != nil {
		return nil, err
	}
	mediatype, payload, err := manifest.Payload()
	if err != nil {
		return nil, err
	}

	if len(d) > 0 {
		dockerImage.ID = d.String()
	} else {
		dockerImage.ID = digest.FromBytes(manifest.Canonical).String()
	}
	image := &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: dockerImage.ID,
		},
		DockerImageMetadata:          *dockerImage,
		DockerImageManifest:          string(payload),
		DockerImageManifestMediaType: mediatype,
		DockerImageMetadataVersion:   "1.0",
	}

	return image, nil
}

func schema2ToImage(manifest *schema2.DeserializedManifest, imageConfig []byte, d digest.Digest) (*api.Image, error) {
	mediatype, payload, err := manifest.Payload()
	if err != nil {
		return nil, err
	}

	dockerImage, err := unmarshalDockerImage(imageConfig)
	if err != nil {
		return nil, err
	}
	if len(d) > 0 {
		dockerImage.ID = d.String()
	} else {
		dockerImage.ID = digest.FromBytes(payload).String()
	}

	image := &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: dockerImage.ID,
		},
		DockerImageMetadata:          *dockerImage,
		DockerImageManifest:          string(payload),
		DockerImageConfig:            string(imageConfig),
		DockerImageManifestMediaType: mediatype,
		DockerImageMetadataVersion:   "1.0",
	}

	return image, nil
}

func schema0ToImage(dockerImage *dockerregistry.Image, id string) (*api.Image, error) {
	var baseImage api.DockerImage
	if err := kapi.Scheme.Convert(&dockerImage.Image, &baseImage, nil); err != nil {
		return nil, fmt.Errorf("could not convert image: %#v", err)
	}

	image := &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: dockerImage.ID,
		},
		DockerImageMetadata:        baseImage,
		DockerImageMetadataVersion: "1.0",
	}

	return image, nil
}

func unmarshalDockerImage(body []byte) (*api.DockerImage, error) {
	var image dockerpre012.DockerImage
	if err := json.Unmarshal(body, &image); err != nil {
		return nil, err
	}
	dockerImage := &api.DockerImage{}
	if err := kapi.Scheme.Convert(&image, dockerImage, nil); err != nil {
		return nil, err
	}
	return dockerImage, nil
}

func isDockerError(err error, code errcode.ErrorCode) bool {
	switch t := err.(type) {
	case errcode.Errors:
		for _, err := range t {
			if isDockerError(err, code) {
				return true
			}
		}
	case errcode.ErrorCode:
		if code == t {
			return true
		}
	case errcode.Error:
		if t.ErrorCode() == code {
			return true
		}
	}
	return false
}

var nowFn = time.Now

type retryRepository struct {
	distribution.Repository

	retries int
	initial *time.Time
	wait    time.Duration
	limit   time.Duration
}

// NewRetryRepository wraps a distribution.Repository with helpers that will retry authentication failures
// over a limited time window and duration. This primarily avoids a DockerHub issue where public images
// unexpectedly return a 401 error due to the JWT token created by the hub being created at the same second,
// but another server being in the previous second.
func NewRetryRepository(repo distribution.Repository, retries int, interval time.Duration) distribution.Repository {
	var wait time.Duration
	if retries > 1 {
		wait = interval / time.Duration(retries-1)
	}
	return &retryRepository{
		Repository: repo,

		retries: retries,
		wait:    wait,
		limit:   interval,
	}
}

// isTemporaryHTTPError returns true if the error indicates a temporary or partial HTTP faliure
func isTemporaryHTTPError(err error) bool {
	if e, ok := err.(net.Error); ok && e != nil {
		return e.Temporary() || e.Timeout()
	}
	return false
}

// shouldRetry returns true if the error is not an unauthorized error, if there are no retries left, or if
// we have already retried once and it has been longer than r.limit since we retried the first time.
func (r *retryRepository) shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	if !isDockerError(err, errcode.ErrorCodeUnauthorized) && !isTemporaryHTTPError(err) {
		return false
	}

	if r.retries <= 0 {
		return false
	}
	r.retries--

	now := nowFn()
	switch {
	case r.initial == nil:
		// always retry the first time immediately
		r.initial = &now
	case r.limit != 0 && now.Sub(*r.initial) > r.limit:
		// give up retrying after the window
		r.retries = 0
	default:
		// don't hot loop
		time.Sleep(r.wait)
	}
	glog.V(4).Infof("Retrying request to a v2 Docker registry after encountering error (%d attempts remaining): %v", r.retries, err)
	return true
}

// Manifests wraps the manifest service in a retryManifest for shared retries.
func (r *retryRepository) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {
	s, err := r.Repository.Manifests(ctx, options...)
	if err != nil {
		return nil, err
	}
	return retryManifest{ManifestService: s, repo: r}, nil
}

// Blobs wraps the blob service in a retryBlobStore for shared retries.
func (r *retryRepository) Blobs(ctx context.Context) distribution.BlobStore {
	return retryBlobStore{BlobStore: r.Repository.Blobs(ctx), repo: r}
}

// Tags lists the tags under the named repository.
func (r *retryRepository) Tags(ctx context.Context) distribution.TagService {
	return &retryTags{TagService: r.Repository.Tags(ctx), repo: r}
}

// retryManifest wraps the manifest service and invokes retries on the repo.
type retryManifest struct {
	distribution.ManifestService
	repo *retryRepository
}

// Exists returns true if the manifest exists.
func (r retryManifest) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	for {
		if exists, err := r.ManifestService.Exists(ctx, dgst); r.repo.shouldRetry(err) {
			continue
		} else {
			return exists, err
		}
	}
}

// Get retrieves the manifest identified by the digest, if it exists.
func (r retryManifest) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	for {
		if m, err := r.ManifestService.Get(ctx, dgst, options...); r.repo.shouldRetry(err) {
			continue
		} else {
			return m, err
		}
	}
}

// retryBlobStore wraps the blob store and invokes retries on the repo.
type retryBlobStore struct {
	distribution.BlobStore
	repo *retryRepository
}

func (r retryBlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	for {
		if d, err := r.BlobStore.Stat(ctx, dgst); r.repo.shouldRetry(err) {
			continue
		} else {
			return d, err
		}
	}
}

func (r retryBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, dgst digest.Digest) error {
	for {
		if err := r.BlobStore.ServeBlob(ctx, w, req, dgst); r.repo.shouldRetry(err) {
			continue
		} else {
			return err
		}
	}
}

func (r retryBlobStore) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	for {
		if rsc, err := r.BlobStore.Open(ctx, dgst); r.repo.shouldRetry(err) {
			continue
		} else {
			return rsc, err
		}
	}
}

type retryTags struct {
	distribution.TagService
	repo *retryRepository
}

func (r *retryTags) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	for {
		if t, err := r.TagService.Get(ctx, tag); r.repo.shouldRetry(err) {
			continue
		} else {
			return t, err
		}
	}
}

func (r *retryTags) All(ctx context.Context) ([]string, error) {
	for {
		if t, err := r.TagService.All(ctx); r.repo.shouldRetry(err) {
			continue
		} else {
			return t, err
		}
	}
}

func (r *retryTags) Lookup(ctx context.Context, digest distribution.Descriptor) ([]string, error) {
	for {
		if t, err := r.TagService.Lookup(ctx, digest); r.repo.shouldRetry(err) {
			continue
		} else {
			return t, err
		}
	}
}
