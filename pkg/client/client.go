package client

import (
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	buildapi "github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/build/api/v1beta1"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	_ "github.com/openshift/origin/pkg/deploy/api/v1beta1"
	imageapi "github.com/openshift/origin/pkg/image/api"
	_ "github.com/openshift/origin/pkg/image/api/v1beta1"
)

// Interface exposes methods on OpenShift resources.
type Interface interface {
	BuildInterface
	BuildConfigInterface
	ImageInterface
	ImageRepositoryInterface
	ImageRepositoryMappingInterface
	DeploymentInterface
	DeploymentConfigInterface
}

// BuildInterface exposes methods on Build resources.
type BuildInterface interface {
	ListBuilds(labels.Selector) (*buildapi.BuildList, error)
	CreateBuild(*buildapi.Build) (*buildapi.Build, error)
	UpdateBuild(*buildapi.Build) (*buildapi.Build, error)
	DeleteBuild(string) error
}

// BuildConfigInterface exposes methods on BuildConfig resources
type BuildConfigInterface interface {
	ListBuildConfigs(labels.Selector) (*buildapi.BuildConfigList, error)
	GetBuildConfig(id string) (*buildapi.BuildConfig, error)
	CreateBuildConfig(*buildapi.BuildConfig) (*buildapi.BuildConfig, error)
	UpdateBuildConfig(*buildapi.BuildConfig) (*buildapi.BuildConfig, error)
	DeleteBuildConfig(string) error
}

// ImageInterface exposes methods on Image resources.
type ImageInterface interface {
	ListImages(labels.Selector) (*imageapi.ImageList, error)
	GetImage(string) (*imageapi.Image, error)
	CreateImage(*imageapi.Image) (*imageapi.Image, error)
}

// ImageRepositoryInterface exposes methods on ImageRepository resources.
type ImageRepositoryInterface interface {
	ListImageRepositories(labels.Selector) (*imageapi.ImageRepositoryList, error)
	GetImageRepository(string) (*imageapi.ImageRepository, error)
	WatchImageRepositories(field, label labels.Selector, resourceVersion uint64) (watch.Interface, error)
	CreateImageRepository(*imageapi.ImageRepository) (*imageapi.ImageRepository, error)
	UpdateImageRepository(*imageapi.ImageRepository) (*imageapi.ImageRepository, error)
}

// ImageRepositoryMappingInterface exposes methods on ImageRepositoryMapping resources.
type ImageRepositoryMappingInterface interface {
	CreateImageRepositoryMapping(*imageapi.ImageRepositoryMapping) error
}

// DeploymentConfigInterface contains methods for working with DeploymentConfigs
type DeploymentConfigInterface interface {
	ListDeploymentConfigs(selector labels.Selector) (*deployapi.DeploymentConfigList, error)
	GetDeploymentConfig(id string) (*deployapi.DeploymentConfig, error)
	CreateDeploymentConfig(*deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
	UpdateDeploymentConfig(*deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
	DeleteDeploymentConfig(string) error
}

// DeploymentInterface contains methods for working with Deployments
type DeploymentInterface interface {
	ListDeployments(selector labels.Selector) (*deployapi.DeploymentList, error)
	GetDeployment(id string) (*deployapi.Deployment, error)
	CreateDeployment(*deployapi.Deployment) (*deployapi.Deployment, error)
	UpdateDeployment(*deployapi.Deployment) (*deployapi.Deployment, error)
	DeleteDeployment(string) error
}

// Client is an OpenShift client object
type Client struct {
	*kubeclient.RESTClient
}

// New creates and returns a new Client.
func New(host string, auth *kubeclient.AuthInfo) (*Client, error) {
	restClient, err := kubeclient.NewRESTClient(host, auth, "/osapi/v1beta1")
	if err != nil {
		return nil, err
	}
	return &Client{restClient}, nil
}

// CreateBuild creates new build. Returns the server's representation of the build and error if one occurs.
func (c *Client) CreateBuild(build *buildapi.Build) (result *buildapi.Build, err error) {
	result = &buildapi.Build{}
	err = c.Post().Path("builds").Body(build).Do().Into(result)
	return
}

// ListBuilds returns a list of builds that match the selector.
func (c *Client) ListBuilds(selector labels.Selector) (result *buildapi.BuildList, err error) {
	result = &buildapi.BuildList{}
	err = c.Get().Path("builds").SelectorParam("labels", selector).Do().Into(result)
	return
}

// UpdateBuild updates the build on server. Returns the server's representation of the build and error if one occurs.
func (c *Client) UpdateBuild(build *buildapi.Build) (result *buildapi.Build, err error) {
	result = &buildapi.Build{}
	err = c.Put().Path("builds").Path(build.ID).Body(build).Do().Into(result)
	return
}

// DeleteBuild deletes a build, returns error if one occurs.
func (c *Client) DeleteBuild(id string) (err error) {
	err = c.Delete().Path("builds").Path(id).Do().Error()
	return
}

// CreateBuildConfig creates a new buildconfig. Returns the server's representation of the buildconfig and error if one occurs.
func (c *Client) CreateBuildConfig(build *buildapi.BuildConfig) (result *buildapi.BuildConfig, err error) {
	result = &buildapi.BuildConfig{}
	err = c.Post().Path("buildConfigs").Body(build).Do().Into(result)
	return
}

// ListBuildConfigs returns a list of buildconfigs that match the selector.
func (c *Client) ListBuildConfigs(selector labels.Selector) (result *buildapi.BuildConfigList, err error) {
	result = &buildapi.BuildConfigList{}
	err = c.Get().Path("buildConfigs").SelectorParam("labels", selector).Do().Into(result)
	return
}

// GetBuildConfig returns information about a particular buildconfig and error if one occurs.
func (c *Client) GetBuildConfig(id string) (result *buildapi.BuildConfig, err error) {
	result = &buildapi.BuildConfig{}
	err = c.Get().Path("buildConfigs").Path(id).Do().Into(result)
	return
}

// UpdateBuildConfig updates the buildconfig on server. Returns the server's representation of the buildconfig and error if one occurs.
func (c *Client) UpdateBuildConfig(build *buildapi.BuildConfig) (result *buildapi.BuildConfig, err error) {
	result = &buildapi.BuildConfig{}
	err = c.Put().Path("buildConfigs").Path(build.ID).Body(build).Do().Into(result)
	return
}

// DeleteBuildConfig deletes a BuildConfig, returns error if one occurs.
func (c *Client) DeleteBuildConfig(id string) error {
	return c.Delete().Path("buildConfigs").Path(id).Do().Error()
}

// ListImages returns a list of images that match the selector.
func (c *Client) ListImages(selector labels.Selector) (result *imageapi.ImageList, err error) {
	result = &imageapi.ImageList{}
	err = c.Get().Path("images").SelectorParam("labels", selector).Do().Into(result)
	return
}

// GetImage returns information about a particular image and error if one occurs.
func (c *Client) GetImage(id string) (result *imageapi.Image, err error) {
	result = &imageapi.Image{}
	err = c.Get().Path("images").Path(id).Do().Into(result)
	return
}

// CreateImage creates a new image. Returns the server's representation of the image and error if one occurs.
func (c *Client) CreateImage(image *imageapi.Image) (result *imageapi.Image, err error) {
	result = &imageapi.Image{}
	err = c.Post().Path("images").Body(image).Do().Into(result)
	return
}

// ListImageRepositories returns a list of imagerepositories that match the selector.
func (c *Client) ListImageRepositories(selector labels.Selector) (result *imageapi.ImageRepositoryList, err error) {
	result = &imageapi.ImageRepositoryList{}
	err = c.Get().Path("imageRepositories").SelectorParam("labels", selector).Do().Into(result)
	return
}

// GetImageRepository returns information about a particular imagerepository and error if one occurs.
func (c *Client) GetImageRepository(id string) (result *imageapi.ImageRepository, err error) {
	result = &imageapi.ImageRepository{}
	err = c.Get().Path("imageRepositories").Path(id).Do().Into(result)
	return
}

// WatchImageRepositories returns a watch.Interface that watches the requested imagerepositories.
func (c *Client) WatchImageRepositories(field, label labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	return c.Get().
		Path("watch").
		Path("imageRepositories").
		UintParam("resourceVersion", resourceVersion).
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Watch()
}

// CreateImageRepository create a new imagerepository. Returns the server's representation of the imagerepository and error if one occurs.
func (c *Client) CreateImageRepository(repo *imageapi.ImageRepository) (result *imageapi.ImageRepository, err error) {
	result = &imageapi.ImageRepository{}
	err = c.Post().Path("imageRepositories").Body(repo).Do().Into(result)
	return
}

// UpdateImageRepository updates the imagerepository on the server. Returns the server's representation of the imagerepository and error if one occurs.
func (c *Client) UpdateImageRepository(repo *imageapi.ImageRepository) (result *imageapi.ImageRepository, err error) {
	result = &imageapi.ImageRepository{}
	err = c.Put().Path("imageRepositories").Path(repo.ID).Body(repo).Do().Into(result)
	return
}

// CreateImageRepositoryMapping create a new imagerepository mapping on the server. Returns error if one occurs.
func (c *Client) CreateImageRepositoryMapping(mapping *imageapi.ImageRepositoryMapping) error {
	return c.Post().Path("imageRepositoryMappings").Body(mapping).Do().Error()
}

// ListDeploymentConfigs takes a selector, and returns the list of deploymentConfigs that match that selector
func (c *Client) ListDeploymentConfigs(selector labels.Selector) (result *deployapi.DeploymentConfigList, err error) {
	result = &deployapi.DeploymentConfigList{}
	err = c.Get().Path("deploymentConfigs").SelectorParam("labels", selector).Do().Into(result)
	return
}

// GetDeploymentConfig returns information about a particular deploymentConfig
func (c *Client) GetDeploymentConfig(id string) (result *deployapi.DeploymentConfig, err error) {
	result = &deployapi.DeploymentConfig{}
	err = c.Get().Path("deploymentConfigs").Path(id).Do().Into(result)
	return
}

// CreateDeploymentConfig creates a new deploymentConfig
func (c *Client) CreateDeploymentConfig(deploymentConfig *deployapi.DeploymentConfig) (result *deployapi.DeploymentConfig, err error) {
	result = &deployapi.DeploymentConfig{}
	err = c.Post().Path("deploymentConfigs").Body(deploymentConfig).Do().Into(result)
	return
}

// UpdateDeploymentConfig updates an existing deploymentConfig
func (c *Client) UpdateDeploymentConfig(deploymentConfig *deployapi.DeploymentConfig) (result *deployapi.DeploymentConfig, err error) {
	result = &deployapi.DeploymentConfig{}
	err = c.Put().Path("deploymentConfigs").Path(deploymentConfig.ID).Body(deploymentConfig).Do().Into(result)
	return
}

// DeleteDeploymentConfig deletes an existing deploymentConfig.
func (c *Client) DeleteDeploymentConfig(id string) error {
	return c.Delete().Path("deploymentConfigs").Path(id).Do().Error()
}

// ListDeployments takes a selector, and returns the list of deployments that match that selector
func (c *Client) ListDeployments(selector labels.Selector) (result *deployapi.DeploymentList, err error) {
	result = &deployapi.DeploymentList{}
	err = c.Get().Path("deployments").SelectorParam("labels", selector).Do().Into(result)
	return
}

// GetDeployment returns information about a particular deployment
func (c *Client) GetDeployment(id string) (result *deployapi.Deployment, err error) {
	result = &deployapi.Deployment{}
	err = c.Get().Path("deployments").Path(id).Do().Into(result)
	return
}

// CreateDeployment creates a new deployment
func (c *Client) CreateDeployment(deployment *deployapi.Deployment) (result *deployapi.Deployment, err error) {
	result = &deployapi.Deployment{}
	err = c.Post().Path("deployments").Body(deployment).Do().Into(result)
	return
}

// UpdateDeployment updates an existing deployment
func (c *Client) UpdateDeployment(deployment *deployapi.Deployment) (result *deployapi.Deployment, err error) {
	result = &deployapi.Deployment{}
	err = c.Put().Path("deployments").Path(deployment.ID).Body(deployment).Do().Into(result)
	return
}

// DeleteDeployment deletes an existing replication deployment.
func (c *Client) DeleteDeployment(id string) error {
	return c.Delete().Path("deployments").Path(id).Do().Error()
}
