package registryclient

import (
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"sort"
	"sync"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/time/rate"

	"k8s.io/klog"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/reference"
	registryclient "github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/opencontainers/go-digest"
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
	Limiter           *rate.Limiter

	DisableDigestVerification bool

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
		Limiter:           c.Limiter,

		DisableDigestVerification: c.DisableDigestVerification,

		pings:    make(map[url.URL]error),
		redirect: make(map[url.URL]*url.URL),
	}
	for k, v := range c.redirect {
		copied.redirect[k] = v
	}
	return copied
}

func (c *Context) WithRateLimiter(limiter *rate.Limiter) *Context {
	c.Limiter = limiter
	return c
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
	if !c.DisableDigestVerification {
		repo = repositoryVerifier{Repository: repo}
	}
	limiter := c.Limiter
	if limiter == nil {
		limiter = rate.NewLimiter(rate.Limit(5), 5)
	}
	return NewLimitedRetryRepository(repo, c.Retries, limiter), nil
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
			klog.V(5).Infof("Falling back to an HTTP check for an insecure registry %s: %v", registry.String(), err)
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
		klog.V(5).Infof("Registry responded to v2 Docker endpoint, but has no header for Docker Distribution %s: %d, %#v", req.URL, resp.StatusCode, resp.Header)
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

	limiter *rate.Limiter
	retries int
	sleepFn func(time.Duration)
}

// NewLimitedRetryRepository wraps a distribution.Repository with helpers that will retry temporary failures
// over a limited time window and duration, and also obeys a rate limit.
func NewLimitedRetryRepository(repo distribution.Repository, retries int, limiter *rate.Limiter) distribution.Repository {
	return &retryRepository{
		Repository: repo,

		limiter: limiter,
		retries: retries,
		sleepFn: time.Sleep,
	}
}

// isTemporaryHTTPError returns true if the error indicates a temporary or partial HTTP failure
func isTemporaryHTTPError(err error) (time.Duration, bool) {
	if err == nil {
		return 0, false
	}
	switch t := err.(type) {
	case net.Error:
		return time.Second, t.Temporary() || t.Timeout()
	case *registryclient.UnexpectedHTTPResponseError:
		if t.StatusCode == http.StatusTooManyRequests {
			return 2 * time.Second, true
		}
	}
	return 0, false
}

// shouldRetry returns true if the error was temporary and count is less than retries.
func (c *retryRepository) shouldRetry(count int, err error) bool {
	if err == nil {
		return false
	}
	retryAfter, ok := isTemporaryHTTPError(err)
	if !ok {
		return false
	}
	if count >= c.retries {
		return false
	}
	c.sleepFn(retryAfter)
	klog.V(4).Infof("Retrying request to Docker registry after encountering error (%d attempts remaining): %v", count, err)
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
func (c retryManifest) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	for i := 0; ; i++ {
		if err := c.repo.limiter.Wait(ctx); err != nil {
			return false, err
		}
		exists, err := c.ManifestService.Exists(ctx, dgst)
		if c.repo.shouldRetry(i, err) {
			continue
		}
		return exists, err
	}
}

// Get retrieves the manifest identified by the digest, if it exists.
func (c retryManifest) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	for i := 0; ; i++ {
		if err := c.repo.limiter.Wait(ctx); err != nil {
			return nil, err
		}
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

func (c retryBlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	for i := 0; ; i++ {
		if err := c.repo.limiter.Wait(ctx); err != nil {
			return distribution.Descriptor{}, err
		}
		d, err := c.BlobStore.Stat(ctx, dgst)
		if c.repo.shouldRetry(i, err) {
			continue
		}
		return d, err
	}
}

func (c retryBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, dgst digest.Digest) error {
	for i := 0; ; i++ {
		if err := c.repo.limiter.Wait(ctx); err != nil {
			return err
		}
		err := c.BlobStore.ServeBlob(ctx, w, req, dgst)
		if c.repo.shouldRetry(i, err) {
			continue
		}
		return err
	}
}

func (c retryBlobStore) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	for i := 0; ; i++ {
		if err := c.repo.limiter.Wait(ctx); err != nil {
			return nil, err
		}
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
		if err := c.repo.limiter.Wait(ctx); err != nil {
			return distribution.Descriptor{}, err
		}
		t, err := c.TagService.Get(ctx, tag)
		if c.repo.shouldRetry(i, err) {
			continue
		}
		return t, err
	}
}

func (c *retryTags) All(ctx context.Context) ([]string, error) {
	for i := 0; ; i++ {
		if err := c.repo.limiter.Wait(ctx); err != nil {
			return nil, err
		}
		t, err := c.TagService.All(ctx)
		if c.repo.shouldRetry(i, err) {
			continue
		}
		return t, err
	}
}

func (c *retryTags) Lookup(ctx context.Context, digest distribution.Descriptor) ([]string, error) {
	for i := 0; ; i++ {
		if err := c.repo.limiter.Wait(ctx); err != nil {
			return nil, err
		}
		t, err := c.TagService.Lookup(ctx, digest)
		if c.repo.shouldRetry(i, err) {
			continue
		}
		return t, err
	}
}

// repositoryVerifier ensures that manifests are verified when they are retrieved via digest
type repositoryVerifier struct {
	distribution.Repository
}

// Manifests returns a ManifestService that checks whether manifests match their digest.
func (r repositoryVerifier) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {
	ms, err := r.Repository.Manifests(ctx, options...)
	if err != nil {
		return nil, err
	}
	return manifestServiceVerifier{ManifestService: ms}, nil
}

// Blobs returns a BlobStore that checks whether blob content returned from the server matches the expected digest.
func (r repositoryVerifier) Blobs(ctx context.Context) distribution.BlobStore {
	return blobStoreVerifier{BlobStore: r.Repository.Blobs(ctx)}
}

// manifestServiceVerifier wraps the manifest service and ensures that content retrieved by digest matches that digest.
type manifestServiceVerifier struct {
	distribution.ManifestService
}

// Get retrieves the manifest identified by the digest and guarantees it matches the content it is retrieved by.
func (m manifestServiceVerifier) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	manifest, err := m.ManifestService.Get(ctx, dgst, options...)
	if err != nil {
		return nil, err
	}
	if len(dgst) > 0 {
		if err := VerifyManifestIntegrity(manifest, dgst); err != nil {
			return nil, err
		}
	}
	return manifest, nil
}

// VerifyManifestIntegrity checks the provided manifest against the specified digest and returns an error
// if the manifest does not match that digest.
func VerifyManifestIntegrity(manifest distribution.Manifest, dgst digest.Digest) error {
	contentDigest, err := ContentDigestForManifest(manifest, dgst.Algorithm())
	if err != nil {
		return err
	}
	if contentDigest != dgst {
		if klog.V(4) {
			_, payload, _ := manifest.Payload()
			klog.Infof("Mismatched content: %s\n%s", contentDigest, string(payload))
		}
		return fmt.Errorf("content integrity error: the manifest retrieved with digest %s does not match the digest calculated from the content %s", dgst, contentDigest)
	}
	return nil
}

// ContentDigestForManifest returns the digest in the provided algorithm of the supplied manifest's contents.
func ContentDigestForManifest(manifest distribution.Manifest, algo digest.Algorithm) (digest.Digest, error) {
	switch t := manifest.(type) {
	case *schema1.SignedManifest:
		// schema1 manifest digests are calculated from the payload
		if len(t.Canonical) == 0 {
			return "", fmt.Errorf("the schema1 manifest does not have a canonical representation")
		}
		return algo.FromBytes(t.Canonical), nil
	default:
		_, payload, err := manifest.Payload()
		if err != nil {
			return "", err
		}
		return algo.FromBytes(payload), nil
	}
}

// blobStoreVerifier wraps the blobs service and ensures that content retrieved by digest matches that digest.
type blobStoreVerifier struct {
	distribution.BlobStore
}

// Get retrieves the blob identified by the digest and guarantees it matches the content it is retrieved by.
func (b blobStoreVerifier) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	data, err := b.BlobStore.Get(ctx, dgst)
	if err != nil {
		return nil, err
	}
	if len(dgst) > 0 {
		dataDgst := dgst.Algorithm().FromBytes(data)
		if dataDgst != dgst {
			return nil, fmt.Errorf("content integrity error: the blob retrieved with digest %s does not match the digest calculated from the content %s", dgst, dataDgst)
		}
	}
	return data, nil
}

// Open streams the blob identified by the digest and guarantees it matches the content it is retrieved by.
func (b blobStoreVerifier) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	rsc, err := b.BlobStore.Open(ctx, dgst)
	if err != nil {
		return nil, err
	}
	if len(dgst) > 0 {
		return &readSeekCloserVerifier{
			rsc:    rsc,
			hash:   dgst.Algorithm().Hash(),
			expect: dgst,
		}, nil
	}
	return rsc, nil
}

// readSeekCloserVerifier performs validation over the stream returned by a distribution.ReadSeekCloser returned
// by blobService.Open.
type readSeekCloserVerifier struct {
	rsc    distribution.ReadSeekCloser
	hash   hash.Hash
	expect digest.Digest
}

// Read verifies the bytes in the underlying stream match the expected digest or returns an error.
func (r *readSeekCloserVerifier) Read(p []byte) (n int, err error) {
	n, err = r.rsc.Read(p)
	if r.hash != nil {
		if n > 0 {
			r.hash.Write(p[:n])
		}
		if err == io.EOF {
			actual := digest.NewDigest(r.expect.Algorithm(), r.hash)
			if actual != r.expect {
				return n, fmt.Errorf("content integrity error: the blob streamed from digest %s does not match the digest calculated from the content %s", r.expect, actual)
			}
		}
	}
	return n, err
}

// Seek moves the underlying stream and also cancels any streaming hash. Verification is not possible
// with a seek.
func (r *readSeekCloserVerifier) Seek(offset int64, whence int) (int64, error) {
	r.hash = nil
	return r.rsc.Seek(offset, whence)
}

// Close closes the underlying stream.
func (r *readSeekCloserVerifier) Close() error {
	return r.rsc.Close()
}
