package dockerregistry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	dockerutils "github.com/docker/docker/utils"
	"github.com/fsouza/go-dockerclient"
	"github.com/smarterclayton/go-dockerregistryclient"
)

// Client includes methods for accessing a Docker registry by name.
type Client interface {
	// Connect to a Docker registry by name. Pass "" for the Docker Hub
	Connect(registry string) (Connection, error)
}

// Connection allows you to retrieve data from a Docker V1 registry.
type Connection interface {
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
		name = registry.IndexServerAddress()
	}
	if conn, ok := c.connections[name]; ok {
		return conn, nil
	}
	e, err := registry.NewEndpoint(name, []string{})
	if err != nil {
		return nil, convertConnectionError(name, err)
	}
	session, err := registry.NewSession(&registry.AuthConfig{}, nil, e, true)
	if err != nil {
		return nil, convertConnectionError(name, err)
	}
	conn := connection{session}
	c.connections[name] = conn
	return conn, nil
}

func convertConnectionError(registry string, err error) error {
	switch {
	case isDockerNotFoundError(err), strings.Contains(err.Error(), "connection refused"):
		return errRegistryNotFound{registry}
	default:
		return err
	}
}

type connection struct {
	*registry.Session
}

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
	repository := fmt.Sprintf("%s/%s", namespace, name)

	data, err := c.GetRepositoryData(repository)
	if err != nil {
		if isDockerNotFoundError(err) {
			return nil, errRepositoryNotFound{repository}
		}
		return nil, err
	}
	tags, err := c.GetRemoteTags(data.Endpoints, repository, data.Tokens)
	if err != nil {
		return nil, err
	}
	id, ok := tags[searchTag]
	if !ok {
		return nil, errTagNotFound{len(tag) == 0, searchTag, repository}
	}
	body, _, err := c.GetRemoteImageJSON(id, data.Endpoints[0], data.Tokens)
	if err != nil {
		if isDockerNotFoundError(err) {
			return nil, errImageNotFound{searchTag, id, repository}
		}
		return nil, err
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

func (e errImageNotFound) Error() string {
	return fmt.Sprintf("the image %q in repository %q with tag %q was not found and may have been deleted", e.tag, e.image, e.repository)
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

func isDockerNotFoundError(err error) bool {
	if json, ok := err.(*dockerutils.JSONError); ok && err != nil {
		return json.Code == http.StatusNotFound
	}
	return false
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
