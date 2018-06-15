package dockerv1client

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	knet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/transport"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// this is the only entrypoint which deals in github.com/fsouza/go-dockerclient.Image and expects to use our conversion capability to coerce an external
// type into an api type.  Localize the crazy here.
// TODO this scheme needs to be configurable or we're going to end up with weird problems.
func init() {
	err := legacyscheme.Scheme.AddConversionFuncs(
		// Convert docker client object to internal object
		func(in *docker.Image, out *imageapi.DockerImage, s conversion.Scope) error {
			if err := s.Convert(&in.Config, &out.Config, conversion.AllowDifferentFieldTypeNames); err != nil {
				return err
			}
			if err := s.Convert(&in.ContainerConfig, &out.ContainerConfig, conversion.AllowDifferentFieldTypeNames); err != nil {
				return err
			}
			out.ID = in.ID
			out.Parent = in.Parent
			out.Comment = in.Comment
			out.Created = metav1.NewTime(in.Created)
			out.Container = in.Container
			out.DockerVersion = in.DockerVersion
			out.Author = in.Author
			out.Architecture = in.Architecture
			out.Size = in.Size
			return nil
		},
		func(in *imageapi.DockerImage, out *docker.Image, s conversion.Scope) error {
			if err := s.Convert(&in.Config, &out.Config, conversion.AllowDifferentFieldTypeNames); err != nil {
				return err
			}
			if err := s.Convert(&in.ContainerConfig, &out.ContainerConfig, conversion.AllowDifferentFieldTypeNames); err != nil {
				return err
			}
			out.ID = in.ID
			out.Parent = in.Parent
			out.Comment = in.Comment
			out.Created = in.Created.Time
			out.Container = in.Container
			out.DockerVersion = in.DockerVersion
			out.Author = in.Author
			out.Architecture = in.Architecture
			out.Size = in.Size
			return nil
		},
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}

type Image struct {
	Image docker.Image

	// Does this registry support pull by ID
	PullByID bool
}

// Client includes methods for accessing a Docker registry by name.
type Client interface {
	// Connect to a Docker registry by name. Pass "" for the Docker Hub
	Connect(registry string, allowInsecure bool) (Connection, error)
}

// Connection allows you to retrieve data from a Docker V1/V2 registry.
type Connection interface {
	// ImageTags will return a map of the tags for the image by namespace and name.
	// If namespace is not specified, will default to "library" for Docker hub.
	ImageTags(namespace, name string) (map[string]string, error)
	// ImageByID will return the requested image by namespace, name, and ID.
	// If namespace is not specified, will default to "library" for Docker hub.
	ImageByID(namespace, name, id string) (*Image, error)
	// ImageByTag will return the requested image by namespace, name, and tag
	// (if not specified, "latest").
	// If namespace is not specified, will default to "library" for Docker hub.
	ImageByTag(namespace, name, tag string) (*Image, error)
	// ImageManifest will return the raw image manifest and digest by namespace,
	// name, and tag.
	ImageManifest(namespace, name, tag string) (string, []byte, error)
}

// client implements the Client interface
type client struct {
	dialTimeout time.Duration
	connections map[string]*connection
	allowV2     bool
}

// NewClient returns a client object which allows public access to
// a Docker registry. enableV2 allows a client to prefer V1 registry
// API connections.
// TODO: accept a docker auth config
func NewClient(dialTimeout time.Duration, allowV2 bool) Client {
	return &client{
		dialTimeout: dialTimeout,
		connections: make(map[string]*connection),
		allowV2:     allowV2,
	}
}

// Connect accepts the name of a registry in the common form Docker provides and will
// create a connection to the registry. Callers may provide a host, a host:port, or
// a fully qualified URL. When not providing a URL, the default scheme will be "https"
func (c *client) Connect(name string, allowInsecure bool) (Connection, error) {
	target, err := normalizeRegistryName(name)
	if err != nil {
		return nil, err
	}
	prefix := target.String()
	if conn, ok := c.connections[prefix]; ok && conn.allowInsecure == allowInsecure {
		return conn, nil
	}
	conn := newConnection(*target, c.dialTimeout, allowInsecure, c.allowV2)
	c.connections[prefix] = conn
	return conn, nil
}

// normalizeDockerHubHost returns the canonical DockerHub registry URL for a given host
// segment and docker API version.
func normalizeDockerHubHost(host string, v2 bool) string {
	switch host {
	case imageapi.DockerDefaultRegistry, "www." + imageapi.DockerDefaultRegistry, imageapi.DockerDefaultV1Registry, imageapi.DockerDefaultV2Registry:
		if v2 {
			return imageapi.DockerDefaultV2Registry
		}
		return imageapi.DockerDefaultV1Registry
	}
	return host
}

// normalizeRegistryName standardizes the registry URL so that it is consistent
// across different versions of the same name (for reuse of auth).
func normalizeRegistryName(name string) (*url.URL, error) {
	prefix := name
	if len(prefix) == 0 {
		prefix = imageapi.DockerDefaultV1Registry
	}
	hadPrefix := false
	switch {
	case strings.HasPrefix(prefix, "http://"), strings.HasPrefix(prefix, "https://"):
		hadPrefix = true
	default:
		prefix = "https://" + prefix
	}

	target, err := url.Parse(prefix)
	if err != nil {
		return nil, fmt.Errorf("the registry name cannot be made into a valid url: %v", err)
	}

	if host, port, err := net.SplitHostPort(target.Host); err == nil {
		host = normalizeDockerHubHost(host, false)
		if hadPrefix {
			switch {
			case port == "443" && target.Scheme == "https":
				target.Host = host
			case port == "80" && target.Scheme == "http":
				target.Host = host
			}
		}
	} else {
		target.Host = normalizeDockerHubHost(target.Host, false)
	}
	return target, nil
}

// convertConnectionError turns a registry error into a typed error if appropriate.
func convertConnectionError(registry string, err error) error {
	switch {
	case strings.Contains(err.Error(), "connection refused"):
		return errRegistryNotFound{registry}
	default:
		return err
	}
}

// connection represents a connection to a particular DockerHub registry, reusing
// tokens and other settings. connections are not thread safe.
type connection struct {
	client *http.Client
	url    url.URL
	cached map[string]repository
	isV2   *bool
	token  string

	allowInsecure bool
}

// newConnection creates a new connection
func newConnection(url url.URL, dialTimeout time.Duration, allowInsecure, enableV2 bool) *connection {
	var isV2 *bool
	if !enableV2 {
		v2 := false
		isV2 = &v2
	}

	var rt http.RoundTripper
	if allowInsecure {
		rt = knet.SetTransportDefaults(&http.Transport{
			Dial: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		})
	} else {
		rt = knet.SetTransportDefaults(&http.Transport{
			Dial: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: 30 * time.Second,
			}).Dial,
		})
	}

	rt = transport.DebugWrappers(rt)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Transport: rt}
	return &connection{
		url:    url,
		client: client,
		cached: make(map[string]repository),
		isV2:   isV2,

		allowInsecure: allowInsecure,
	}
}

// ImageTags returns the tags for the named Docker image repository.
func (c *connection) ImageTags(namespace, name string) (map[string]string, error) {
	if len(namespace) == 0 && imageapi.IsRegistryDockerHub(c.url.Host) {
		namespace = imageapi.DockerDefaultNamespace
	}
	if len(name) == 0 {
		return nil, fmt.Errorf("image name must be specified")
	}

	repo, err := c.getCachedRepository(fmt.Sprintf("%s/%s", namespace, name))
	if err != nil {
		return nil, err
	}

	return repo.getTags(c)
}

// ImageByID returns the specified image within the named Docker image repository
func (c *connection) ImageByID(namespace, name, imageID string) (*Image, error) {
	if len(namespace) == 0 && imageapi.IsRegistryDockerHub(c.url.Host) {
		namespace = imageapi.DockerDefaultNamespace
	}
	if len(name) == 0 {
		return nil, fmt.Errorf("image name must be specified")
	}

	repo, err := c.getCachedRepository(fmt.Sprintf("%s/%s", namespace, name))
	if err != nil {
		return nil, err
	}

	image, _, err := repo.getImage(c, imageID, "")
	return image, err
}

// ImageByTag returns the specified image within the named Docker image repository
func (c *connection) ImageByTag(namespace, name, tag string) (*Image, error) {
	if len(namespace) == 0 && imageapi.IsRegistryDockerHub(c.url.Host) {
		namespace = imageapi.DockerDefaultNamespace
	}
	if len(name) == 0 {
		return nil, fmt.Errorf("image name must be specified")
	}
	searchTag := tag
	if len(searchTag) == 0 {
		searchTag = imageapi.DefaultImageTag
	}

	repo, err := c.getCachedRepository(fmt.Sprintf("%s/%s", namespace, name))
	if err != nil {
		return nil, err
	}

	image, _, err := repo.getTaggedImage(c, searchTag, tag)
	return image, err
}

// ImageManifest returns raw manifest of the specified image within the named Docker image repository
func (c *connection) ImageManifest(namespace, name, tag string) (string, []byte, error) {
	if len(name) == 0 {
		return "", nil, fmt.Errorf("image name must be specified")
	}
	if len(namespace) == 0 && imageapi.IsRegistryDockerHub(c.url.Host) {
		namespace = imageapi.DockerDefaultNamespace
	}
	searchTag := tag
	if len(searchTag) == 0 {
		searchTag = imageapi.DefaultImageTag
	}

	repo, err := c.getCachedRepository(fmt.Sprintf("%s/%s", namespace, name))
	if err != nil {
		return "", nil, err
	}

	image, manifest, err := repo.getTaggedImage(c, searchTag, tag)
	if err != nil {
		return "", nil, err
	}
	return image.Image.ID, manifest, err
}

// getCachedRepository returns a repository interface matching the provided name and
// may cache information about the server on the connection object.
func (c *connection) getCachedRepository(name string) (repository, error) {
	if cached, ok := c.cached[name]; ok {
		return cached, nil
	}

	if c.isV2 == nil {
		v2, err := c.checkV2()
		if err != nil {
			return nil, err
		}
		c.isV2 = &v2
	}
	if *c.isV2 {
		base := c.url
		base.Host = normalizeDockerHubHost(base.Host, true)
		repo := &v2repository{
			name:     name,
			endpoint: base,
			token:    c.token,
		}
		c.cached[name] = repo
		return repo, nil
	}

	repo, err := c.getRepositoryV1(name)
	if err != nil {
		return nil, err
	}
	c.cached[name] = repo
	return repo, nil
}

// checkV2 performs the registry version checking steps as described by
// https://docs.docker.com/registry/spec/api/
func (c *connection) checkV2() (bool, error) {
	base := c.url
	base.Host = normalizeDockerHubHost(base.Host, true)
	base.Path = path.Join(base.Path, "v2") + "/"
	req, err := http.NewRequest("GET", base.String(), nil)
	if err != nil {
		return false, fmt.Errorf("error creating request: %v", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		// if we tried https and were rejected, try http
		if c.url.Scheme == "https" && c.allowInsecure {
			glog.V(4).Infof("Failed to get https, trying http: %v", err)
			c.url.Scheme = "http"
			return c.checkV2()
		}
		return false, convertConnectionError(c.url.String(), fmt.Errorf("error checking for V2 registry at %s: %v", base.String(), err))
	}
	defer resp.Body.Close()

	switch code := resp.StatusCode; {
	case code == http.StatusUnauthorized:
		// handle auth challenges on individual repositories
	case code >= 300 || resp.StatusCode < 200:
		return false, nil
	}
	if len(resp.Header.Get("Docker-Distribution-API-Version")) == 0 {
		glog.V(5).Infof("Registry v2 API at %s did not have a Docker-Distribution-API-Version header", base.String())
		return false, nil
	}

	glog.V(5).Infof("Found registry v2 API at %s", base.String())
	return true, nil
}

// parseAuthChallenge splits a header of the form 'type[ <key>="<value>"[,...]]' returned
// by the docker registry
func parseAuthChallenge(header string) (string, map[string]string) {
	sections := strings.SplitN(header, " ", 2)
	if len(sections) == 1 {
		sections = append(sections, "")
	}
	challenge := sections[1]
	keys := make(map[string]string)
	for _, s := range strings.Split(challenge, ",") {
		pair := strings.SplitN(strings.TrimSpace(s), "=", 2)
		if len(pair) == 1 {
			keys[pair[0]] = ""
			continue
		}
		keys[pair[0]] = strings.Trim(pair[1], "\"")
	}
	return sections[0], keys
}

// authenticateV2 attempts to respond to a given WWW-Authenticate challenge header
// by asking for a token from the realm. Currently only supports "Bearer" challenges
// with no credentials provided.
// TODO: support credentials or replace with the Docker distribution v2 registry client
func (c *connection) authenticateV2(header string) (string, error) {
	mode, keys := parseAuthChallenge(header)
	if strings.ToLower(mode) != "bearer" {
		return "", fmt.Errorf("unsupported authentication challenge from registry: %s", header)
	}

	realm, ok := keys["realm"]
	if !ok {
		return "", fmt.Errorf("no realm specified by the server, cannot authenticate: %s", header)
	}
	delete(keys, "realm")

	realmURL, err := url.Parse(realm)
	if err != nil {
		return "", fmt.Errorf("realm %q was not a valid url: %v", realm, err)
	}
	query := realmURL.Query()
	for k, v := range keys {
		query.Set(k, v)
	}
	realmURL.RawQuery = query.Encode()
	req, err := http.NewRequest("GET", realmURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("error creating v2 auth request: %v", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", convertConnectionError(realmURL.String(), fmt.Errorf("error authorizing to the registry: %v", err))
	}
	defer resp.Body.Close()

	switch code := resp.StatusCode; {
	case code == http.StatusUnauthorized:
		return "", fmt.Errorf("permission denied to access realm %q", realmURL.String())
	case code == http.StatusNotFound:
		return "", fmt.Errorf("defined realm %q cannot be found", realm)
	case code >= 300 || resp.StatusCode < 200:
		return "", fmt.Errorf("error authenticating to the realm %q; server returned %d", realmURL.String(), resp.StatusCode)
	}

	token := struct {
		Token string `json:"token"`
	}{}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("can't read authorization body from %s: %v", realmURL.String(), err)
	}
	if err := json.Unmarshal(body, &token); err != nil {
		return "", fmt.Errorf("can't decode the server authorization from %s: %v", realmURL.String(), err)
	}
	return token.Token, nil
}

// getRepositoryV1 returns a repository implementation for a v1 registry by asking for
// the appropriate endpoint token. It will try HTTP if HTTPS fails and insecure connections
// are allowed.
func (c *connection) getRepositoryV1(name string) (repository, error) {
	glog.V(4).Infof("Getting repository %s from %s", name, c.url.String())

	base := c.url
	base.Path = path.Join(base.Path, fmt.Sprintf("/v1/repositories/%s/images", name))
	req, err := http.NewRequest("GET", base.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Add("X-Docker-Token", "true")
	resp, err := c.client.Do(req)
	if err != nil {
		// if we tried https and were rejected, try http
		if c.url.Scheme == "https" && c.allowInsecure {
			glog.V(4).Infof("Failed to get https, trying http: %v", err)
			c.url.Scheme = "http"
			return c.getRepositoryV1(name)
		}
		return nil, convertConnectionError(c.url.String(), fmt.Errorf("error getting X-Docker-Token from %s: %v", name, err))
	}
	defer resp.Body.Close()

	// if we were redirected, update the base urls
	c.url.Scheme = resp.Request.URL.Scheme
	c.url.Host = resp.Request.URL.Host

	switch code := resp.StatusCode; {
	case code == http.StatusNotFound:
		return nil, errRepositoryNotFound{name}
	case code >= 300 || resp.StatusCode < 200:
		return nil, fmt.Errorf("error retrieving repository: server returned %d", resp.StatusCode)
	}

	// TODO: select a random endpoint
	return &v1repository{
		name:     name,
		endpoint: url.URL{Scheme: c.url.Scheme, Host: resp.Header.Get("X-Docker-Endpoints")},
		token:    resp.Header.Get("X-Docker-Token"),
	}, nil
}

// repository is an interface for retrieving image info from a Docker V1 or V2 repository.
type repository interface {
	getTags(c *connection) (map[string]string, error)
	getTaggedImage(c *connection, tag, userTag string) (*Image, []byte, error)
	getImage(c *connection, image, userTag string) (*Image, []byte, error)
}

// v2repository exposes methods for accessing a named Docker V2 repository on a server.
type v2repository struct {
	name     string
	endpoint url.URL
	token    string
	retries  int
}

// v2tags describes the tags/list returned by the Docker V2 registry.
type v2tags struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func (repo *v2repository) getTags(c *connection) (map[string]string, error) {
	endpoint := repo.endpoint
	endpoint.Path = path.Join(endpoint.Path, fmt.Sprintf("/v2/%s/tags/list", repo.name))
	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	addAcceptHeader(req)

	if len(repo.token) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", repo.token))
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, convertConnectionError(c.url.String(), fmt.Errorf("error getting image tags for %s: %v", repo.name, err))
	}
	defer resp.Body.Close()

	switch code := resp.StatusCode; {
	case code == http.StatusUnauthorized:
		if len(repo.token) != 0 {
			// The DockerHub returns JWT tokens that take effect at "now" at second resolution, which means clients can
			// be rejected when requests are made near the time boundary.
			if repo.retries > 0 {
				repo.retries--
				time.Sleep(time.Second / 2)
				return repo.getTags(c)
			}
			delete(c.cached, repo.name)
			// docker will not return a NotFound on any repository URL - for backwards compatibility, return NotFound on the
			// repo
			return nil, errRepositoryNotFound{repo.name}
		}
		token, err := c.authenticateV2(resp.Header.Get("WWW-Authenticate"))
		if err != nil {
			return nil, fmt.Errorf("error getting image tags for %s: %v", repo.name, err)
		}
		repo.retries = 2
		repo.token = token
		return repo.getTags(c)

	case code == http.StatusNotFound:
		return nil, errRepositoryNotFound{repo.name}
	case code >= 300 || resp.StatusCode < 200:
		// token might have expired - evict repo from cache so we can get a new one on retry
		delete(c.cached, repo.name)
		return nil, fmt.Errorf("error retrieving tags: server returned %d", resp.StatusCode)
	}
	tags := &v2tags{}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("error decoding image %s tags: %v", repo.name, err)
	}
	legacyTags := make(map[string]string)
	for _, tag := range tags.Tags {
		legacyTags[tag] = tag
	}
	return legacyTags, nil
}

func (repo *v2repository) getTaggedImage(c *connection, tag, userTag string) (*Image, []byte, error) {
	endpoint := repo.endpoint
	endpoint.Path = path.Join(endpoint.Path, fmt.Sprintf("/v2/%s/manifests/%s", repo.name, tag))
	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating request: %v", err)
	}
	addAcceptHeader(req)

	if len(repo.token) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", repo.token))
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, convertConnectionError(c.url.String(), fmt.Errorf("error getting image for %s:%s: %v", repo.name, tag, err))
	}
	defer resp.Body.Close()

	switch code := resp.StatusCode; {
	case code == http.StatusUnauthorized:
		if len(repo.token) != 0 {
			// The DockerHub returns JWT tokens that take effect at "now" at second resolution, which means clients can
			// be rejected when requests are made near the time boundary.
			if repo.retries > 0 {
				repo.retries--
				time.Sleep(time.Second / 2)
				return repo.getTaggedImage(c, tag, userTag)
			}
			delete(c.cached, repo.name)
			// docker will not return a NotFound on any repository URL - for backwards compatibility, return NotFound on the
			// repo
			body, _ := ioutil.ReadAll(resp.Body)
			glog.V(4).Infof("passed valid auth token, but unable to find tagged image at %q, %d %v: %s", req.URL.String(), resp.StatusCode, resp.Header, body)
			return nil, nil, errTagNotFound{len(userTag) == 0, tag, repo.name}
		}
		token, err := c.authenticateV2(resp.Header.Get("WWW-Authenticate"))
		if err != nil {
			return nil, nil, fmt.Errorf("error getting image for %s:%s: %v", repo.name, tag, err)
		}
		repo.retries = 2
		repo.token = token
		return repo.getTaggedImage(c, tag, userTag)
	case code == http.StatusNotFound:
		body, _ := ioutil.ReadAll(resp.Body)
		glog.V(4).Infof("unable to find tagged image at %q, %d %v: %s", req.URL.String(), resp.StatusCode, resp.Header, body)
		return nil, nil, errTagNotFound{len(userTag) == 0, tag, repo.name}
	case code >= 300 || resp.StatusCode < 200:
		// token might have expired - evict repo from cache so we can get a new one on retry
		delete(c.cached, repo.name)

		return nil, nil, fmt.Errorf("error retrieving tagged image: server returned %d", resp.StatusCode)
	}

	digest := resp.Header.Get("Docker-Content-Digest")

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("can't read image body from %s: %v", req.URL, err)
	}
	dockerImage, err := repo.unmarshalImageManifest(c, body)
	if err != nil {
		return nil, nil, err
	}
	image := &Image{
		Image: *dockerImage,
	}
	if len(digest) > 0 {
		image.Image.ID = digest
		image.PullByID = true
	}
	return image, body, nil
}

func (repo *v2repository) getImage(c *connection, image, userTag string) (*Image, []byte, error) {
	return repo.getTaggedImage(c, image, userTag)
}

func (repo *v2repository) getImageConfig(c *connection, dgst string) ([]byte, error) {
	endpoint := repo.endpoint
	endpoint.Path = path.Join(endpoint.Path, fmt.Sprintf("/v2/%s/blobs/%s", repo.name, dgst))
	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	if len(repo.token) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", repo.token))
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, convertConnectionError(c.url.String(), fmt.Errorf("error getting image config for %s: %v", repo.name, err))
	}
	defer resp.Body.Close()

	switch code := resp.StatusCode; {
	case code == http.StatusUnauthorized:
		if len(repo.token) != 0 {
			// The DockerHub returns JWT tokens that take effect at "now" at second resolution, which means clients can
			// be rejected when requests are made near the time boundary.
			if repo.retries > 0 {
				repo.retries--
				time.Sleep(time.Second / 2)
				return repo.getImageConfig(c, dgst)
			}
			delete(c.cached, repo.name)
			// docker will not return a NotFound on any repository URL - for backwards compatibility, return NotFound on the
			// repo
			body, _ := ioutil.ReadAll(resp.Body)
			glog.V(4).Infof("passed valid auth token, but unable to find image config at %q, %d %v: %s", req.URL.String(), resp.StatusCode, resp.Header, body)
			return nil, errBlobNotFound{dgst, repo.name}
		}
		token, err := c.authenticateV2(resp.Header.Get("WWW-Authenticate"))
		if err != nil {
			return nil, fmt.Errorf("error getting image config for %s:%s: %v", repo.name, dgst, err)
		}
		repo.retries = 2
		repo.token = token
		return repo.getImageConfig(c, dgst)
	case code == http.StatusNotFound:
		body, _ := ioutil.ReadAll(resp.Body)
		glog.V(4).Infof("unable to find image config at %q, %d %v: %s", req.URL.String(), resp.StatusCode, resp.Header, body)
		return nil, errBlobNotFound{dgst, repo.name}
	case code >= 300 || resp.StatusCode < 200:
		// token might have expired - evict repo from cache so we can get a new one on retry
		delete(c.cached, repo.name)

		return nil, fmt.Errorf("error retrieving image config: server returned %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("can't read image body from %s: %v", req.URL, err)
	}

	return body, nil
}

func (repo *v2repository) unmarshalImageManifest(c *connection, body []byte) (*docker.Image, error) {
	manifest := imageapi.DockerImageManifest{}
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, err
	}
	switch manifest.SchemaVersion {
	case 1:
		if len(manifest.History) == 0 {
			return nil, fmt.Errorf("image has no v1Compatibility history and cannot be used")
		}
		return unmarshalDockerImage([]byte(manifest.History[0].DockerV1Compatibility))
	case 2:
		config, err := repo.getImageConfig(c, manifest.Config.Digest)
		if err != nil {
			return nil, err
		}
		return unmarshalDockerImage(config)
	}
	return nil, fmt.Errorf("unrecognized Docker image manifest schema %d", manifest.SchemaVersion)
}

// v1repository exposes methods for accessing a named Docker V1 repository on a server.
type v1repository struct {
	name     string
	endpoint url.URL
	token    string
}

func (repo *v1repository) getTags(c *connection) (map[string]string, error) {
	endpoint := repo.endpoint
	endpoint.Path = path.Join(endpoint.Path, fmt.Sprintf("/v1/repositories/%s/tags", repo.name))
	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Add("Authorization", "Token "+repo.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, convertConnectionError(c.url.String(), fmt.Errorf("error getting image tags for %s: %v", repo.name, err))
	}
	defer resp.Body.Close()

	switch code := resp.StatusCode; {
	case code == http.StatusNotFound:
		return nil, errRepositoryNotFound{repo.name}
	case code >= 300 || resp.StatusCode < 200:
		// token might have expired - evict repo from cache so we can get a new one on retry
		delete(c.cached, repo.name)

		return nil, fmt.Errorf("error retrieving tags: server returned %d", resp.StatusCode)
	}
	tags := make(map[string]string)
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("error decoding image %s tags: %v", repo.name, err)
	}
	return tags, nil
}

func (repo *v1repository) getTaggedImage(c *connection, tag, userTag string) (*Image, []byte, error) {
	endpoint := repo.endpoint
	endpoint.Path = path.Join(endpoint.Path, fmt.Sprintf("/v1/repositories/%s/tags/%s", repo.name, tag))
	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Add("Authorization", "Token "+repo.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, convertConnectionError(c.url.String(), fmt.Errorf("error getting image id for %s:%s: %v", repo.name, tag, err))
	}
	defer resp.Body.Close()

	switch code := resp.StatusCode; {
	case code == http.StatusNotFound:
		// Attempt to lookup tag in tags map, supporting registries that don't allow retrieval
		// of tags to ids (Pulp/Crane)
		allTags, err := repo.getTags(c)
		if err != nil {
			return nil, nil, err
		}
		if image, ok := allTags[tag]; ok {
			return repo.getImage(c, image, "")
		}
		body, _ := ioutil.ReadAll(resp.Body)
		glog.V(4).Infof("unable to find v1 tagged image at %q, %d %v: %s", req.URL.String(), resp.StatusCode, resp.Header, body)
		return nil, nil, errTagNotFound{len(userTag) == 0, tag, repo.name}
	case code >= 300 || resp.StatusCode < 200:
		// token might have expired - evict repo from cache so we can get a new one on retry
		delete(c.cached, repo.name)

		return nil, nil, fmt.Errorf("error retrieving tag: server returned %d", resp.StatusCode)
	}
	var imageID string
	if err := json.NewDecoder(resp.Body).Decode(&imageID); err != nil {
		return nil, nil, fmt.Errorf("error decoding image id: %v", err)
	}
	return repo.getImage(c, imageID, "")
}

func (repo *v1repository) getImage(c *connection, image, userTag string) (*Image, []byte, error) {
	endpoint := repo.endpoint
	endpoint.Path = path.Join(endpoint.Path, fmt.Sprintf("/v1/images/%s/json", image))
	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating request: %v", err)
	}

	if len(repo.token) > 0 {
		req.Header.Add("Authorization", "Token "+repo.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, convertConnectionError(c.url.String(), fmt.Errorf("error getting json for image %q: %v", image, err))
	}
	defer resp.Body.Close()
	switch code := resp.StatusCode; {
	case code == http.StatusNotFound:
		return nil, nil, NewImageNotFoundError(repo.name, image, userTag)
	case code >= 300 || resp.StatusCode < 200:
		// token might have expired - evict repo from cache so we can get a new one on retry
		delete(c.cached, repo.name)
		if body, err := ioutil.ReadAll(resp.Body); err == nil {
			glog.V(6).Infof("unable to fetch image %s: %#v\n%s", req.URL, resp, string(body))
		}
		return nil, nil, fmt.Errorf("error retrieving image %s: server returned %d", req.URL, resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("can't read image body from %s: %v", req.URL, err)
	}
	dockerImage, err := unmarshalDockerImage(body)
	if err != nil {
		return nil, nil, err
	}
	return &Image{Image: *dockerImage}, body, nil
}

// errBlobNotFound is an error indicating the requested blob does not exist in the repository.
type errBlobNotFound struct {
	digest     string
	repository string
}

func (e errBlobNotFound) Error() string {
	return fmt.Sprintf("blob %s was not found in repository %q", e.digest, e.repository)
}

// errTagNotFound is an error indicating the requested tag does not exist on the server. May be returned on
// a v2 repository when the repository does not exist (because the v2 registry returns 401 on any repository
// you do not have permission to see, or does not exist)
type errTagNotFound struct {
	wasDefault bool
	tag        string
	repository string
}

func (e errTagNotFound) Error() string {
	if e.wasDefault {
		return fmt.Sprintf("the default tag %q has not been set on repository %q", e.tag, e.repository)
	}
	return fmt.Sprintf("tag %q has not been set on repository %q", e.tag, e.repository)
}

// errRepositoryNotFound indicates the repository is not found - but is only guaranteed to be returned
// for v1 docker registries.
type errRepositoryNotFound struct {
	repository string
}

func (e errRepositoryNotFound) Error() string {
	return fmt.Sprintf("the repository %q was not found", e.repository)
}

type errImageNotFound struct {
	tag        string
	image      string
	repository string
}

func NewImageNotFoundError(repository, image, tag string) error {
	return errImageNotFound{tag, image, repository}
}

func (e errImageNotFound) Error() string {
	if len(e.tag) == 0 {
		return fmt.Sprintf("the image %q in repository %q was not found and may have been deleted", e.image, e.repository)
	}
	return fmt.Sprintf("the image %q in repository %q with tag %q was not found and may have been deleted", e.image, e.repository, e.tag)
}

type errRegistryNotFound struct {
	registry string
}

func (e errRegistryNotFound) Error() string {
	return fmt.Sprintf("the registry %q could not be reached", e.registry)
}

func IsRegistryNotFound(err error) bool {
	_, ok := err.(errRegistryNotFound)
	return ok
}

func IsRepositoryNotFound(err error) bool {
	_, ok := err.(errRepositoryNotFound)
	return ok
}

func IsImageNotFound(err error) bool {
	_, ok := err.(errImageNotFound)
	return ok
}

func IsTagNotFound(err error) bool {
	_, ok := err.(errTagNotFound)
	return ok
}

func IsBlobNotFound(err error) bool {
	_, ok := err.(errBlobNotFound)
	return ok
}

func IsNotFound(err error) bool {
	return IsRegistryNotFound(err) || IsRepositoryNotFound(err) || IsImageNotFound(err) || IsTagNotFound(err) || IsBlobNotFound(err)
}

func unmarshalDockerImage(body []byte) (*docker.Image, error) {
	var imagePre012 docker.ImagePre012
	if err := json.Unmarshal(body, &imagePre012); err != nil {
		return nil, err
	}

	return &docker.Image{
		ID:              imagePre012.ID,
		Parent:          imagePre012.Parent,
		Comment:         imagePre012.Comment,
		Created:         imagePre012.Created,
		Container:       imagePre012.Container,
		ContainerConfig: imagePre012.ContainerConfig,
		DockerVersion:   imagePre012.DockerVersion,
		Author:          imagePre012.Author,
		Config:          imagePre012.Config,
		Architecture:    imagePre012.Architecture,
		Size:            imagePre012.Size,
	}, nil
}

func addAcceptHeader(r *http.Request) {
	r.Header.Add("Accept", schema1.MediaTypeManifest)
	r.Header.Add("Accept", schema2.MediaTypeManifest)
}
