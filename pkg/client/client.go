package client

import (
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"

	"github.com/openshift/origin/pkg/api/latest"
)

// Interface exposes methods on OpenShift resources.
type Interface interface {
	BuildsNamespacer
	BuildConfigsNamespacer
	ImagesNamespacer
	ImageRepositoriesNamespacer
	ImageRepositoryMappingsNamespacer
	DeploymentsNamespacer
	DeploymentConfigsNamespacer
	RoutesNamespacer
	UsersInterface
	UserIdentityMappingsInterface
	ProjectsInterface
}

func (c *Client) Builds(namespace string) BuildInterface {
	return newBuilds(c, namespace)
}

func (c *Client) BuildConfigs(namespace string) BuildConfigInterface {
	return newBuildConfigs(c, namespace)
}

func (c *Client) Images(namespace string) ImageInterface {
	return newImages(c, namespace)
}

func (c *Client) ImageRepositories(namespace string) ImageRepositoryInterface {
	return newImageRepositories(c, namespace)
}

func (c *Client) ImageRepositoryMappings(namespace string) ImageRepositoryMappingInterface {
	return newImageRepositoryMappings(c, namespace)
}

func (c *Client) Deployments(namespace string) DeploymentInterface {
	return newDeployments(c, namespace)
}

func (c *Client) DeploymentConfigs(namespace string) DeploymentConfigInterface {
	return newDeploymentConfigs(c, namespace)
}

func (c *Client) Routes(namespace string) RouteInterface {
	return newRoutes(c, namespace)
}

func (c *Client) Users() UserInterface {
	return newUsers(c)
}

func (c *Client) UserIdentityMappings() UserIdentityMappingInterface {
	return newUserIdentityMappings(c)
}

func (c *Client) Projects() ProjectInterface {
	return newProjects(c)
}

// Client is an OpenShift client object
type Client struct {
	*kclient.RESTClient
}

// New creates an OpenShift client for the given config. This client works with builds, deployments,
// templates, routes, and images. It allows operations such as list, get, update and delete on these
// objects. An error is returned if the provided configuration is not valid.
func New(c *kclient.Config) (*Client, error) {
	config := *c
	if config.Prefix == "" {
		config.Prefix = "/osapi"
	}
	if config.Version == "" {
		// Clients default to the preferred code API version
		// TODO: implement version negotiation (highest version supported by server)
		config.Version = latest.Version
	}
	client, err := kclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &Client{client}, nil
}

// NewOrDie creates an OpenShift client and panics if the provided API version is not recognized.
func NewOrDie(c *kclient.Config) *Client {
	client, err := New(c)
	if err != nil {
		panic(err)
	}
	return client
}
