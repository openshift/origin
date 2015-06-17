package dockerregistry

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// Client includes methods for accessing a Docker registry by name.
type Client interface {
	// Connect to a Docker registry by name. Pass "" for the Docker Hub
	Connect(registry string, allowInsecure bool) (Connection, error)
}

// Connection allows you to retrieve data from a Docker V1 registry.
type Connection interface {
	// ImageTags will return a map of the tags for the image by namespace (if not
	// specified, will be "library") and name.
	ImageTags(namespace, name string) (map[string]string, error)
	// ImageByID will return the requested image by namespace (if not specified,
	// will be "library"), name, and ID.
	ImageByID(namespace, name, id string) (*docker.Image, error)
	// ImageByTag will return the requested image by namespace (if not specified,
	// will be "library"), name, and tag (if not specified, "latest").
	ImageByTag(namespace, name, tag string) (*docker.Image, error)
}

// NewClient returns a client object which allows public access to
// a Docker registry.
// TODO: accept a docker auth config
func NewClient() Client {
	return &client{
		connections: make(map[string]*connection),
	}
}

// client implements the Client interface
type client struct {
	connections map[string]*connection
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
	conn := newConnection(*target, allowInsecure)
	c.connections[prefix] = conn
	return conn, nil
}

func normalizeRegistryName(name string) (*url.URL, error) {
	prefix := name
	if len(prefix) == 0 {
		prefix = "index.docker.io"
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
		if host == "docker.io" {
			host = "index.docker.io"
		}
		if hadPrefix {
			switch {
			case port == "443" && target.Scheme == "https":
				target.Host = host
			case port == "80" && target.Scheme == "http":
				target.Host = host
			}
		}
	} else {
		if target.Host == "docker.io" {
			target.Host = "index.docker.io"
		}
	}
	return target, nil
}

func convertConnectionError(registry string, err error) error {
	switch {
	case strings.Contains(err.Error(), "connection refused"):
		return errRegistryNotFound{registry}
	default:
		return err
	}
}

type connection struct {
	client *http.Client
	url    url.URL
	cached map[string]*repository

	allowInsecure bool
}

func newConnection(url url.URL, allowInsecure bool) *connection {
	client := http.DefaultClient
	if allowInsecure {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy:           http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
		}
		client = &http.Client{Transport: tr}
	}
	return &connection{
		url:    url,
		client: client,
		cached: make(map[string]*repository),

		allowInsecure: allowInsecure,
	}
}

type repository struct {
	name     string
	endpoint url.URL
	token    string
}

// ImageTags returns the tags for the named Docker image repository.
func (c *connection) ImageTags(namespace, name string) (map[string]string, error) {
	if len(namespace) == 0 {
		namespace = "library"
	}
	if len(name) == 0 {
		return nil, fmt.Errorf("image name must be specified")
	}

	repo, err := c.getCachedRepository(fmt.Sprintf("%s/%s", namespace, name))
	if err != nil {
		return nil, err
	}

	return c.getTags(repo)
}

// ImageByID returns the specified image within the named Docker image repository
func (c *connection) ImageByID(namespace, name, imageID string) (*docker.Image, error) {
	if len(namespace) == 0 {
		namespace = "library"
	}
	if len(name) == 0 {
		return nil, fmt.Errorf("image name must be specified")
	}

	repo, err := c.getCachedRepository(fmt.Sprintf("%s/%s", namespace, name))
	if err != nil {
		return nil, err
	}

	return c.getImage(repo, imageID, "")
}

// ImageByTag returns the specified image within the named Docker image repository
func (c *connection) ImageByTag(namespace, name, tag string) (*docker.Image, error) {
	if len(namespace) == 0 {
		namespace = "library"
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

	imageID, err := c.getTag(repo, searchTag, tag)
	if err != nil {
		return nil, err
	}

	return c.getImage(repo, imageID, tag)
}

func (c *connection) getCachedRepository(name string) (*repository, error) {
	if cached, ok := c.cached[name]; ok {
		return cached, nil
	}
	repo, err := c.getRepository(name)
	if err != nil {
		return nil, err
	}
	c.cached[name] = repo
	return repo, nil
}

func (c *connection) getRepository(name string) (*repository, error) {
	glog.V(4).Infof("Getting repository %s from %s", name, c.url)
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
			return c.getRepository(name)
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
	return &repository{
		name:     name,
		endpoint: url.URL{Scheme: c.url.Scheme, Host: resp.Header.Get("X-Docker-Endpoints")},
		token:    resp.Header.Get("X-Docker-Token"),
	}, nil
}

func (c *connection) getTags(repo *repository) (map[string]string, error) {
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

func (c *connection) getTag(repo *repository, tag, userTag string) (string, error) {
	endpoint := repo.endpoint
	endpoint.Path = path.Join(endpoint.Path, fmt.Sprintf("/v1/repositories/%s/tags/%s", repo.name, tag))
	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Add("Authorization", "Token "+repo.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return "", convertConnectionError(c.url.String(), fmt.Errorf("error getting image id for %s:%s: %v", repo.name, tag, err))
	}
	defer resp.Body.Close()

	switch code := resp.StatusCode; {
	case code == http.StatusNotFound:
		return "", errTagNotFound{len(userTag) == 0, tag, repo.name}
	case code >= 300 || resp.StatusCode < 200:
		// token might have expired - evict repo from cache so we can get a new one on retry
		delete(c.cached, repo.name)

		return "", fmt.Errorf("error retrieving tag: server returned %d", resp.StatusCode)
	}
	var imageID string
	if err := json.NewDecoder(resp.Body).Decode(&imageID); err != nil {
		return "", fmt.Errorf("error decoding image id: %v", err)
	}
	return imageID, nil
}

func (c *connection) getImage(repo *repository, image, userTag string) (*docker.Image, error) {
	endpoint := repo.endpoint
	endpoint.Path = path.Join(endpoint.Path, fmt.Sprintf("/v1/images/%s/json", image))
	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Add("Authorization", "Token "+repo.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, convertConnectionError(c.url.String(), fmt.Errorf("error getting json for image %q: %v", image, err))
	}
	switch code := resp.StatusCode; {
	case code == http.StatusNotFound:
		return nil, NewImageNotFoundError(repo.name, image, userTag)
	case code >= 300 || resp.StatusCode < 200:
		// token might have expired - evict repo from cache so we can get a new one on retry
		delete(c.cached, repo.name)

		return nil, fmt.Errorf("error retrieving image %s: server returned %d", req.URL, resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("can't read image body from %s: %v", req.URL, err)
	}
	return unmarshalDockerImage(body)
}

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

func IsNotFound(err error) bool {
	return IsRegistryNotFound(err) || IsRepositoryNotFound(err) || IsImageNotFound(err) || IsTagNotFound(err)
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
