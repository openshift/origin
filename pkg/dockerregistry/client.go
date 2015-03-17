package dockerregistry

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/fsouza/go-dockerclient"
)

// Client includes methods for accessing a Docker registry by name.
type Client interface {
	// Connect to a Docker registry by name. Pass "" for the Docker Hub
	Connect(registry string) (Connection, error)
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
func NewClient() Client {
	return &client{
		connections: make(map[string]connection),
	}
}

// client implements the Client interface
type client struct {
	connections map[string]connection
}

func (c *client) Connect(name string) (Connection, error) {
	if len(name) == 0 {
		name = "index.docker.io"
	}
	if conn, ok := c.connections[name]; ok {
		return conn, nil
	}
	conn := newConnection(name)
	c.connections[name] = conn
	return conn, nil
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
	host   string
	cached map[string]*repository
}

func newConnection(name string) connection {
	return connection{
		host:   name,
		client: http.DefaultClient,
		cached: make(map[string]*repository),
	}
}

type repository struct {
	name     string
	endpoint string
	token    string
}

// ImageTags returns the tags for the named Docker image repository.
func (c connection) ImageTags(namespace, name string) (map[string]string, error) {
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
func (c connection) ImageByID(namespace, name, imageID string) (*docker.Image, error) {
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
func (c connection) ImageByTag(namespace, name, tag string) (*docker.Image, error) {
	if len(namespace) == 0 {
		namespace = "library"
	}
	if len(name) == 0 {
		return nil, fmt.Errorf("image name must be specified")
	}
	searchTag := tag
	if len(searchTag) == 0 {
		searchTag = "latest"
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

func (c connection) getCachedRepository(name string) (*repository, error) {
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

func (c connection) getRepository(name string) (*repository, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/v1/repositories/%s/images", c.host, name), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Add("X-Docker-Token", "true")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, convertConnectionError(c.host, fmt.Errorf("error getting X-Docker-Token from index.docker.io: %v", err))
	}
	switch code := resp.StatusCode; {
	case code == http.StatusNotFound:
		return nil, errRepositoryNotFound{name}
	case code >= 300 || resp.StatusCode < 200:
		return nil, fmt.Errorf("error retrieving repository: server returned %d", resp.StatusCode)
	}
	return &repository{
		name:     name,
		endpoint: resp.Header.Get("X-Docker-Endpoints"),
		token:    resp.Header.Get("X-Docker-Token"),
	}, nil
}

func (c connection) getTags(repo *repository) (map[string]string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/v1/repositories/%s/tags", repo.endpoint, repo.name), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Add("Authorization", "Token "+repo.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, convertConnectionError(c.host, fmt.Errorf("error getting image tags for %s: %v", repo.name, err))
	}
	switch code := resp.StatusCode; {
	case code == http.StatusNotFound:
		return nil, errRepositoryNotFound{repo.name}
	case code >= 300 || resp.StatusCode < 200:
		return nil, fmt.Errorf("error retrieving tags: server returned %d", resp.StatusCode)
	}
	tags := make(map[string]string)
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("error decoding image %s tags: %v", repo.name, err)
	}
	return tags, nil
}

func (c connection) getTag(repo *repository, tag, userTag string) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/v1/repositories/%s/tags/%s", repo.endpoint, repo.name, tag), nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Add("Authorization", "Token "+repo.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return "", convertConnectionError(c.host, fmt.Errorf("error getting image id for %s:%s: %v", repo.name, tag, err))
	}
	switch code := resp.StatusCode; {
	case code == http.StatusNotFound:
		return "", errTagNotFound{len(userTag) == 0, tag, repo.name}
	case code >= 300 || resp.StatusCode < 200:
		return "", fmt.Errorf("error retrieving tag: server returned %d", resp.StatusCode)
	}
	var imageID string
	if err := json.NewDecoder(resp.Body).Decode(&imageID); err != nil {
		return "", fmt.Errorf("error decoding image id: %v", err)
	}
	return imageID, nil
}

func (c connection) getImage(repo *repository, image, userTag string) (*docker.Image, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/v1/images/%s/json", repo.endpoint, image), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Add("Authorization", "Token "+repo.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, convertConnectionError(c.host, fmt.Errorf("error getting json for image %q: %v", image, err))
	}
	switch code := resp.StatusCode; {
	case code == http.StatusNotFound:
		return nil, NewImageNotFoundError(repo.name, image, userTag)
	case code >= 300 || resp.StatusCode < 200:
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
