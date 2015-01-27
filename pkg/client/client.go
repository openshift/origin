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
	ImageRepositoryTagsNamespacer
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

// ImageRepositories provides a REST client for ImageRepository
func (c *Client) ImageRepositories(namespace string) ImageRepositoryInterface {
	return newImageRepositories(c, namespace)
}

// ImageRepositoryMappings provides a REST client for ImageRepositoryMapping
func (c *Client) ImageRepositoryMappings(namespace string) ImageRepositoryMappingInterface {
	return newImageRepositoryMappings(c, namespace)
}

// ImageRepositoryTags provides a REST client for ImageRepositoryTag
func (c *Client) ImageRepositoryTags(namespace string) ImageRepositoryTagInterface {
	return newImageRepositoryTags(c, namespace)
}

// Deployments provides a REST client for Deployment
func (c *Client) Deployments(namespace string) DeploymentInterface {
	return newDeployments(c, namespace)
}

// DeploymentConfigs provides a REST client for DeploymentConfig
func (c *Client) DeploymentConfigs(namespace string) DeploymentConfigInterface {
	return newDeploymentConfigs(c, namespace)
}

// Routes provides a REST client for Route
func (c *Client) Routes(namespace string) RouteInterface {
	return newRoutes(c, namespace)
}

// Users provides a REST client for User
func (c *Client) Users() UserInterface {
	return newUsers(c)
}

// UserIdentityMappings provides a REST client for UserIdentityMapping
func (c *Client) UserIdentityMappings() UserIdentityMappingInterface {
	return newUserIdentityMappings(c)
}

// Projects provides a REST client for Projects
func (c *Client) Projects() ProjectInterface {
	return newProjects(c)
}

// TemplateConfigs provides a REST client for TemplateConfig
func (c *Client) TemplateConfigs(namespace string) TemplateConfigInterface {
	return newTemplateConfigs(c, namespace)
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
	config.Codec = latest.Codec
	config.LegacyBehavior = (config.Version == "v1beta1")
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
