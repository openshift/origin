package registryclient

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"sort"
	"sync"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/docker/distribution"
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
	Repository(ctx context.Context, registry *url.URL, repoName string, insecure bool) (distribution.Repository, error)
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

type transportCache struct {
	rt        http.RoundTripper
	scopes    map[string]struct{}
	transport http.RoundTripper
}

type Context struct {
	Transport         http.RoundTripper
	InsecureTransport http.RoundTripper
	Challenges        challenge.Manager
	Scopes            []auth.Scope
	Actions           []string
	Retries           int
	Credentials       auth.CredentialStore

	lock             sync.Mutex
	pings            map[url.URL]error
	redirect         map[url.URL]*url.URL
	cachedTransports []transportCache
}

func (c *Context) Copy() *Context {
	c.lock.Lock()
	defer c.lock.Unlock()
	copied := &Context{
		Transport:         c.Transport,
		InsecureTransport: c.InsecureTransport,
		Challenges:        c.Challenges,
		Scopes:            c.Scopes,
		Actions:           c.Actions,
		Retries:           c.Retries,
		Credentials:       c.Credentials,

		pings:    make(map[url.URL]error),
		redirect: make(map[url.URL]*url.URL),
	}
	for k, v := range c.redirect {
		copied.redirect[k] = v
	}
	return copied
}

func (c *Context) WithScopes(scopes ...auth.Scope) *Context {
	c.Scopes = scopes
	return c
}

func (c *Context) WithActions(actions ...string) *Context {
	c.Actions = actions
	return c
}

func (c *Context) WithCredentials(credentials auth.CredentialStore) *Context {
	c.Credentials = credentials
	return c
}

// Reset clears any cached repository info for this context.
func (c *Context) Reset() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.pings = nil
	c.redirect = nil
}

func (c *Context) cachedPing(src url.URL) (*url.URL, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	err, ok := c.pings[src]
	if !ok {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if redirect, ok := c.redirect[src]; ok {
		src = *redirect
	}
	return &src, nil
}

// Ping contacts a registry and returns the transport and URL of the registry or an error.
func (c *Context) Ping(ctx context.Context, registry *url.URL, insecure bool) (http.RoundTripper, *url.URL, error) {
	t := c.Transport
	if insecure && c.InsecureTransport != nil {
		t = c.InsecureTransport
	}
	src := *registry
	if len(src.Scheme) == 0 {
		src.Scheme = "https"
	}

	// reused cached pings
	url, err := c.cachedPing(src)
	if err != nil {
		return nil, nil, err
	}
	if url != nil {
		return t, url, nil
	}

	// follow redirects
	redirect, err := c.ping(src, insecure, t)

	c.lock.Lock()
	defer c.lock.Unlock()
	c.pings[src] = err
	if err != nil {
		return nil, nil, err
	}
	if redirect != nil {
		c.redirect[src] = redirect
		src = *redirect
	}
	return t, &src, nil
}

func (c *Context) Repository(ctx context.Context, registry *url.URL, repoName string, insecure bool) (distribution.Repository, error) {
	named, err := reference.WithName(repoName)
	if err != nil {
		return nil, err
	}

	rt, src, err := c.Ping(ctx, registry, insecure)
	if err != nil {
		return nil, err
	}

	rt = c.repositoryTransport(rt, src, repoName)

	repo, err := registryclient.NewRepository(named, src.String(), rt)
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

func hasAll(a, b map[string]struct{}) bool {
	for key := range b {
		if _, ok := a[key]; !ok {
			return false
		}
	}
	return true
}

type stringScope string

func (s stringScope) String() string { return string(s) }

// cachedTransport reuses an underlying transport for the given round tripper based
// on the set of passed scopes. It will always return a transport that has at least the
// provided scope list.
func (c *Context) cachedTransport(rt http.RoundTripper, scopes []auth.Scope) http.RoundTripper {
	scopeNames := make(map[string]struct{})
	for _, scope := range scopes {
		scopeNames[scope.String()] = struct{}{}
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	for _, c := range c.cachedTransports {
		if c.rt == rt && hasAll(c.scopes, scopeNames) {
			return c.transport
		}
	}

	// avoid taking a dependency on kube sets.String for minimal dependencies
	names := make([]string, 0, len(scopeNames))
	for s := range scopeNames {
		names = append(names, s)
	}
	sort.Strings(names)
	scopes = make([]auth.Scope, 0, len(scopeNames))
	for _, s := range names {
		scopes = append(scopes, stringScope(s))
	}

	t := transport.NewTransport(
		rt,
		// TODO: slightly smarter authorizer that retries unauthenticated requests
		// TODO: make multiple attempts if the first credential fails
		auth.NewAuthorizer(
			c.Challenges,
			auth.NewTokenHandlerWithOptions(auth.TokenHandlerOptions{
				Transport:   rt,
				Credentials: c.Credentials,
				Scopes:      scopes,
			}),
			auth.NewBasicHandler(c.Credentials),
		),
	)
	c.cachedTransports = append(c.cachedTransports, transportCache{
		rt:        rt,
		scopes:    scopeNames,
		transport: t,
	})
	return t
}

func (c *Context) scopes(repoName string) []auth.Scope {
	scopes := make([]auth.Scope, 0, 1+len(c.Scopes))
	scopes = append(scopes, c.Scopes...)
	if len(c.Actions) == 0 {
		scopes = append(scopes, auth.RepositoryScope{Repository: repoName, Actions: []string{"pull"}})
	} else {
		scopes = append(scopes, auth.RepositoryScope{Repository: repoName, Actions: c.Actions})
	}
	return scopes
}

func (c *Context) repositoryTransport(t http.RoundTripper, registry *url.URL, repoName string) http.RoundTripper {
	return c.cachedTransport(t, c.scopes(repoName))
}

var nowFn = time.Now

type retryRepository struct {
	distribution.Repository

	retries int
	wait    time.Duration
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
	}
}

// isTemporaryHTTPError returns true if the error indicates a temporary or partial HTTP failure
func isTemporaryHTTPError(err error) bool {
	if e, ok := err.(net.Error); ok && e != nil {
		return e.Temporary() || e.Timeout()
	}
	return false
}

// shouldRetry returns true if the error was temporary and count is less than retries.
func (c *retryRepository) shouldRetry(count int, err error) bool {
	if err == nil {
		return false
	}
	if !isTemporaryHTTPError(err) {
		return false
	}
	if count >= c.retries {
		return false
	}
	// don't hot loop
	time.Sleep(c.wait)
	glog.V(4).Infof("Retrying request to Docker registry after encountering error (%d attempts remaining): %v", count, err)
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
	for i := 0; ; i++ {
		exists, err := c.ManifestService.Exists(ctx, dgst)
		if c.repo.shouldRetry(i, err) {
			continue
		}
		return exists, err
	}
}

// Get retrieves the manifest identified by the digest, if it exists.
func (c retryManifest) Get(ctx context.Context, dgst godigest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	for i := 0; ; i++ {
		m, err := c.ManifestService.Get(ctx, dgst, options...)
		if c.repo.shouldRetry(i, err) {
			continue
		}
		return m, err
	}
}

// retryBlobStore wraps the blob store and invokes retries on the repo.
type retryBlobStore struct {
	distribution.BlobStore
	repo *retryRepository
}

func (c retryBlobStore) Stat(ctx context.Context, dgst godigest.Digest) (distribution.Descriptor, error) {
	for i := 0; ; i++ {
		d, err := c.BlobStore.Stat(ctx, dgst)
		if c.repo.shouldRetry(i, err) {
			continue
		}
		return d, err
	}
}

func (c retryBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, dgst godigest.Digest) error {
	for i := 0; ; i++ {
		err := c.BlobStore.ServeBlob(ctx, w, req, dgst)
		if c.repo.shouldRetry(i, err) {
			continue
		}
		return err
	}
}

func (c retryBlobStore) Open(ctx context.Context, dgst godigest.Digest) (distribution.ReadSeekCloser, error) {
	for i := 0; ; i++ {
		rsc, err := c.BlobStore.Open(ctx, dgst)
		if c.repo.shouldRetry(i, err) {
			continue
		}
		return rsc, err
	}
}

type retryTags struct {
	distribution.TagService
	repo *retryRepository
}

func (c *retryTags) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	for i := 0; ; i++ {
		t, err := c.TagService.Get(ctx, tag)
		if c.repo.shouldRetry(i, err) {
			continue
		}
		return t, err
	}
}

func (c *retryTags) All(ctx context.Context) ([]string, error) {
	for i := 0; ; i++ {
		t, err := c.TagService.All(ctx)
		if c.repo.shouldRetry(i, err) {
			continue
		}
		return t, err
	}
}

func (c *retryTags) Lookup(ctx context.Context, digest distribution.Descriptor) ([]string, error) {
	for i := 0; ; i++ {
		t, err := c.TagService.Lookup(ctx, digest)
		if c.repo.shouldRetry(i, err) {
			continue
		}
		return t, err
	}
}
