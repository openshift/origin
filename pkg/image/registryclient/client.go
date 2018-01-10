package registryclient

import (
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
	"github.com/docker/distribution/reference"
	registryclient "github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/docker/distribution/registry/client/transport"
	godigest "github.com/opencontainers/go-digest"
)

// RepositoryRetriever fetches a Docker distribution.Repository.
type RepositoryRetriever interface {
	// Repository returns a properly authenticated distribution.Repository for the given registry, repository
	// name, and insecure toleration behavior.
	Repository(ctx gocontext.Context, registry *url.URL, repoName string, insecure bool) (distribution.Repository, error)
}

// ErrNotV2Registry is returned when the server does not report itself as a V2 Docker registry
type ErrNotV2Registry struct {
	Registry string
}

func (e *ErrNotV2Registry) Error() string {
	return fmt.Sprintf("endpoint %q does not support v2 API", e.Registry)
}

type AuthHandlersFunc func(transport http.RoundTripper, registry *url.URL, repoName string) []auth.AuthenticationHandler

// NewContext is capable of creating RepositoryRetrievers.
func NewContext(transport, insecureTransport http.RoundTripper) *Context {
	return &Context{
		Transport:         transport,
		InsecureTransport: insecureTransport,
		Challenges:        challenge.NewSimpleManager(),
		Actions:           []string{"pull"},
		Retries:           2,
		Credentials:       NoCredentials,

		pings:    make(map[url.URL]error),
		redirect: make(map[url.URL]*url.URL),
	}
}

type Context struct {
	Transport         http.RoundTripper
	InsecureTransport http.RoundTripper
	Challenges        challenge.Manager
	Scopes            []auth.Scope
	Actions           []string
	Retries           int
	Credentials       auth.CredentialStore

	authFn   AuthHandlersFunc
	pings    map[url.URL]error
	redirect map[url.URL]*url.URL
}

func (c *Context) WithScopes(scopes ...auth.Scope) *Context {
	c.authFn = nil
	c.Scopes = scopes
	return c
}

func (c *Context) WithActions(actions ...string) *Context {
	c.authFn = nil
	c.Actions = actions
	return c
}

func (c *Context) WithCredentials(credentials auth.CredentialStore) *Context {
	c.authFn = nil
	c.Credentials = credentials
	return c
}

func (c *Context) wrapTransport(t http.RoundTripper, registry *url.URL, repoName string) http.RoundTripper {
	if c.authFn == nil {
		c.authFn = func(rt http.RoundTripper, _ *url.URL, repoName string) []auth.AuthenticationHandler {
			scopes := make([]auth.Scope, 0, 1+len(c.Scopes))
			scopes = append(scopes, c.Scopes...)
			if len(c.Actions) == 0 {
				scopes = append(scopes, auth.RepositoryScope{Repository: repoName, Actions: []string{"pull"}})
			} else {
				scopes = append(scopes, auth.RepositoryScope{Repository: repoName, Actions: c.Actions})
			}
			return []auth.AuthenticationHandler{
				auth.NewTokenHandlerWithOptions(auth.TokenHandlerOptions{
					Transport:   rt,
					Credentials: c.Credentials,
					Scopes:      scopes,
				}),
				auth.NewBasicHandler(c.Credentials),
			}
		}
	}
	return transport.NewTransport(
		t,
		// TODO: slightly smarter authorizer that retries unauthenticated requests
		// TODO: make multiple attempts if the first credential fails
		auth.NewAuthorizer(
			c.Challenges,
			c.authFn(t, registry, repoName)...,
		),
	)
}

func (c *Context) Repository(ctx gocontext.Context, registry *url.URL, repoName string, insecure bool) (distribution.Repository, error) {
	named, err := reference.WithName(repoName)
	if err != nil {
		return nil, err
	}

	t := c.Transport
	if insecure && c.InsecureTransport != nil {
		t = c.InsecureTransport
	}
	src := *registry
	if len(src.Scheme) == 0 {
		src.Scheme = "https"
	}

	// ping the registry to get challenge headers
	if err, ok := c.pings[src]; ok {
		if err != nil {
			return nil, err
		}
		if redirect, ok := c.redirect[src]; ok {
			src = *redirect
		}
	} else {
		redirect, err := c.ping(src, insecure, t)
		c.pings[src] = err
		if err != nil {
			return nil, err
		}
		if redirect != nil {
			c.redirect[src] = redirect
			src = *redirect
		}
	}

	rt := c.wrapTransport(t, registry, repoName)

	repo, err := registryclient.NewRepository(context.Context(ctx), named, src.String(), rt)
	if err != nil {
		return nil, err
	}
	if c.Retries > 0 {
		return NewRetryRepository(repo, c.Retries, 3/2*time.Second), nil
	}
	return repo, nil
}

func (c *Context) ping(registry url.URL, insecure bool, transport http.RoundTripper) (*url.URL, error) {
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
			glog.V(5).Infof("Falling back to an HTTP check for an insecure registry %s: %v", registry.String(), err)
			registry.Scheme = "http"
			_, nErr := c.ping(registry, true, transport)
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
		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			// v2
		case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
			// v2
		default:
			return nil, &ErrNotV2Registry{Registry: registry.String()}
		}
	}

	c.Challenges.AddResponse(resp)

	return nil, nil
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

// isTemporaryHTTPError returns true if the error indicates a temporary or partial HTTP failure
func isTemporaryHTTPError(err error) bool {
	if e, ok := err.(net.Error); ok && e != nil {
		return e.Temporary() || e.Timeout()
	}
	return false
}

// shouldRetry returns true if the error is not an unauthorized error, if there are no retries left, or if
// we have already retried once and it has been longer than c.limit since we retried the first time.
func (c *retryRepository) shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	if !isTemporaryHTTPError(err) {
		return false
	}

	if c.retries <= 0 {
		return false
	}
	c.retries--

	now := nowFn()
	switch {
	case c.initial == nil:
		// always retry the first time immediately
		c.initial = &now
	case c.limit != 0 && now.Sub(*c.initial) > c.limit:
		// give up retrying after the window
		c.retries = 0
	default:
		// don't hot loop
		time.Sleep(c.wait)
	}
	glog.V(4).Infof("Retrying request to a v2 Docker registry after encountering error (%d attempts remaining): %v", c.retries, err)
	return true
}

// Manifests wraps the manifest service in a retryManifest for shared retries.
func (c *retryRepository) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {
	s, err := c.Repository.Manifests(ctx, options...)
	if err != nil {
		return nil, err
	}
	return retryManifest{ManifestService: s, repo: c}, nil
}

// Blobs wraps the blob service in a retryBlobStore for shared retries.
func (c *retryRepository) Blobs(ctx context.Context) distribution.BlobStore {
	return retryBlobStore{BlobStore: c.Repository.Blobs(ctx), repo: c}
}

// Tags lists the tags under the named repository.
func (c *retryRepository) Tags(ctx context.Context) distribution.TagService {
	return &retryTags{TagService: c.Repository.Tags(ctx), repo: c}
}

// retryManifest wraps the manifest service and invokes retries on the repo.
type retryManifest struct {
	distribution.ManifestService
	repo *retryRepository
}

// Exists returns true if the manifest exists.
func (c retryManifest) Exists(ctx context.Context, dgst godigest.Digest) (bool, error) {
	for {
		if exists, err := c.ManifestService.Exists(ctx, dgst); c.repo.shouldRetry(err) {
			continue
		} else {
			return exists, err
		}
	}
}

// Get retrieves the manifest identified by the digest, if it exists.
func (c retryManifest) Get(ctx context.Context, dgst godigest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	for {
		if m, err := c.ManifestService.Get(ctx, dgst, options...); c.repo.shouldRetry(err) {
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

func (c retryBlobStore) Stat(ctx context.Context, dgst godigest.Digest) (distribution.Descriptor, error) {
	for {
		if d, err := c.BlobStore.Stat(ctx, dgst); c.repo.shouldRetry(err) {
			continue
		} else {
			return d, err
		}
	}
}

func (c retryBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, dgst godigest.Digest) error {
	for {
		if err := c.BlobStore.ServeBlob(ctx, w, req, dgst); c.repo.shouldRetry(err) {
			continue
		} else {
			return err
		}
	}
}

func (c retryBlobStore) Open(ctx context.Context, dgst godigest.Digest) (distribution.ReadSeekCloser, error) {
	for {
		if rsc, err := c.BlobStore.Open(ctx, dgst); c.repo.shouldRetry(err) {
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

func (c *retryTags) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	for {
		if t, err := c.TagService.Get(ctx, tag); c.repo.shouldRetry(err) {
			continue
		} else {
			return t, err
		}
	}
}

func (c *retryTags) All(ctx context.Context) ([]string, error) {
	for {
		if t, err := c.TagService.All(ctx); c.repo.shouldRetry(err) {
			continue
		} else {
			return t, err
		}
	}
}

func (c *retryTags) Lookup(ctx context.Context, digest distribution.Descriptor) ([]string, error) {
	for {
		if t, err := c.TagService.Lookup(ctx, digest); c.repo.shouldRetry(err) {
			continue
		} else {
			return t, err
		}
	}
}
