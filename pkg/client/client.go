package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
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
	RouteInterface
}

// BuildInterface exposes methods on Build resources.
type BuildInterface interface {
	ListBuilds(ctx api.Context, labels labels.Selector) (*buildapi.BuildList, error)
	CreateBuild(ctx api.Context, build *buildapi.Build) (*buildapi.Build, error)
	UpdateBuild(ctx api.Context, build *buildapi.Build) (*buildapi.Build, error)
	DeleteBuild(ctx api.Context, id string) error
}

// BuildConfigInterface exposes methods on BuildConfig resources
type BuildConfigInterface interface {
	ListBuildConfigs(ctx api.Context, labels labels.Selector) (*buildapi.BuildConfigList, error)
	GetBuildConfig(ctx api.Context, id string) (*buildapi.BuildConfig, error)
	CreateBuildConfig(ctx api.Context, config *buildapi.BuildConfig) (*buildapi.BuildConfig, error)
	UpdateBuildConfig(ctx api.Context, config *buildapi.BuildConfig) (*buildapi.BuildConfig, error)
	DeleteBuildConfig(ctx api.Context, id string) error
}

// ImageInterface exposes methods on Image resources.
type ImageInterface interface {
	ListImages(ctx api.Context, labels labels.Selector) (*imageapi.ImageList, error)
	GetImage(ctx api.Context, id string) (*imageapi.Image, error)
	CreateImage(ctx api.Context, image *imageapi.Image) (*imageapi.Image, error)
}

// ImageRepositoryInterface exposes methods on ImageRepository resources.
type ImageRepositoryInterface interface {
	ListImageRepositories(ctx api.Context, labels labels.Selector) (*imageapi.ImageRepositoryList, error)
	GetImageRepository(ctx api.Context, id string) (*imageapi.ImageRepository, error)
	WatchImageRepositories(ctx api.Context, field, label labels.Selector, resourceVersion uint64) (watch.Interface, error)
	CreateImageRepository(ctx api.Context, repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error)
	UpdateImageRepository(ctx api.Context, repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error)
}

// ImageRepositoryMappingInterface exposes methods on ImageRepositoryMapping resources.
type ImageRepositoryMappingInterface interface {
	CreateImageRepositoryMapping(ctx api.Context, mapping *imageapi.ImageRepositoryMapping) error
}

// DeploymentConfigInterface contains methods for working with DeploymentConfigs
type DeploymentConfigInterface interface {
	ListDeploymentConfigs(ctx api.Context, selector labels.Selector) (*deployapi.DeploymentConfigList, error)
	WatchDeploymentConfigs(ctx api.Context, field, label labels.Selector, resourceVersion uint64) (watch.Interface, error)
	GetDeploymentConfig(ctx api.Context, id string) (*deployapi.DeploymentConfig, error)
	CreateDeploymentConfig(ctx api.Context, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
	UpdateDeploymentConfig(ctx api.Context, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
	DeleteDeploymentConfig(ctx api.Context, id string) error
	GenerateDeploymentConfig(ctx api.Context, id string) (*deployapi.DeploymentConfig, error)
}

// DeploymentInterface contains methods for working with Deployments
type DeploymentInterface interface {
	ListDeployments(ctx api.Context, selector labels.Selector) (*deployapi.DeploymentList, error)
	GetDeployment(ctx api.Context, id string) (*deployapi.Deployment, error)
	CreateDeployment(ctx api.Context, deployment *deployapi.Deployment) (*deployapi.Deployment, error)
	UpdateDeployment(ctx api.Context, deployment *deployapi.Deployment) (*deployapi.Deployment, error)
	DeleteDeployment(ctx api.Context, id string) error
	WatchDeployments(ctx api.Context, field, label labels.Selector, resourceVersion uint64) (watch.Interface, error)
}

// RouteInterface exposes methods on Route resources
type RouteInterface interface {
	ListRoutes(ctx api.Context, selector labels.Selector) (*routeapi.RouteList, error)
	GetRoute(ctx api.Context, id string) (*routeapi.Route, error)
	CreateRoute(ctx api.Context, route *routeapi.Route) (*routeapi.Route, error)
	UpdateRoute(ctx api.Context, route *routeapi.Route) (*routeapi.Route, error)
	DeleteRoute(ctx api.Context, id string) error
	WatchRoutes(ctx api.Context, label, field labels.Selector, resourceVersion uint64) (watch.Interface, error)
}

// Client is an OpenShift client object
type Client struct {
	*kubeclient.RESTClient
}

// New creates an OpenShift client for the given config. This client works with builds, deployments,
// templates, routes, and images. It allows operations such as list, get, update and delete on these
// objects. An error is returned if the provided configuration is not valid.
func New(c *kubeclient.Config) (*Client, error) {
	config := *c
	if config.Prefix == "" {
		config.Prefix = "/osapi"
	}
	if config.Version == "" {
		// Clients default to the preferred code API version
		// TODO: implement version negotiation (highest version supported by server)
		config.Version = latest.Version
	}
	client, err := kubeclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &Client{client}, nil
}

// NewOrDie creates an OpenShift client and panics if the provided API version is not recognized.
func NewOrDie(c *kubeclient.Config) *Client {
	client, err := New(c)
	if err != nil {
		panic(err)
	}
	return client
}

// CreateBuild creates new build. Returns the server's representation of the build and error if one occurs.
func (c *Client) CreateBuild(ctx api.Context, build *buildapi.Build) (result *buildapi.Build, err error) {
	result = &buildapi.Build{}
	err = c.Post().Path("builds").Body(build).Do().Into(result)
	return
}

// ListBuilds returns a list of builds that match the selector.
func (c *Client) ListBuilds(ctx api.Context, selector labels.Selector) (result *buildapi.BuildList, err error) {
	result = &buildapi.BuildList{}
	err = c.Get().Path("builds").SelectorParam("labels", selector).Do().Into(result)
	return
}

// UpdateBuild updates the build on server. Returns the server's representation of the build and error if one occurs.
func (c *Client) UpdateBuild(ctx api.Context, build *buildapi.Build) (result *buildapi.Build, err error) {
	result = &buildapi.Build{}
	err = c.Put().Path("builds").Path(build.ID).Body(build).Do().Into(result)
	return
}

// DeleteBuild deletes a build, returns error if one occurs.
func (c *Client) DeleteBuild(ctx api.Context, id string) (err error) {
	err = c.Delete().Path("builds").Path(id).Do().Error()
	return
}

// CreateBuildConfig creates a new buildconfig. Returns the server's representation of the buildconfig and error if one occurs.
func (c *Client) CreateBuildConfig(ctx api.Context, build *buildapi.BuildConfig) (result *buildapi.BuildConfig, err error) {
	result = &buildapi.BuildConfig{}
	err = c.Post().Path("buildConfigs").Body(build).Do().Into(result)
	return
}

// ListBuildConfigs returns a list of buildconfigs that match the selector.
func (c *Client) ListBuildConfigs(ctx api.Context, selector labels.Selector) (result *buildapi.BuildConfigList, err error) {
	result = &buildapi.BuildConfigList{}
	err = c.Get().Path("buildConfigs").SelectorParam("labels", selector).Do().Into(result)
	return
}

// GetBuildConfig returns information about a particular buildconfig and error if one occurs.
func (c *Client) GetBuildConfig(ctx api.Context, id string) (result *buildapi.BuildConfig, err error) {
	result = &buildapi.BuildConfig{}
	err = c.Get().Path("buildConfigs").Path(id).Do().Into(result)
	return
}

// UpdateBuildConfig updates the buildconfig on server. Returns the server's representation of the buildconfig and error if one occurs.
func (c *Client) UpdateBuildConfig(ctx api.Context, build *buildapi.BuildConfig) (result *buildapi.BuildConfig, err error) {
	result = &buildapi.BuildConfig{}
	err = c.Put().Path("buildConfigs").Path(build.ID).Body(build).Do().Into(result)
	return
}

// DeleteBuildConfig deletes a BuildConfig, returns error if one occurs.
func (c *Client) DeleteBuildConfig(ctx api.Context, id string) error {
	return c.Delete().Path("buildConfigs").Path(id).Do().Error()
}

// ListImages returns a list of images that match the selector.
func (c *Client) ListImages(ctx api.Context, selector labels.Selector) (result *imageapi.ImageList, err error) {
	result = &imageapi.ImageList{}
	err = c.Get().Path("images").SelectorParam("labels", selector).Do().Into(result)
	return
}

// GetImage returns information about a particular image and error if one occurs.
func (c *Client) GetImage(ctx api.Context, id string) (result *imageapi.Image, err error) {
	result = &imageapi.Image{}
	err = c.Get().Path("images").Path(id).Do().Into(result)
	return
}

// CreateImage creates a new image. Returns the server's representation of the image and error if one occurs.
func (c *Client) CreateImage(ctx api.Context, image *imageapi.Image) (result *imageapi.Image, err error) {
	result = &imageapi.Image{}
	err = c.Post().Path("images").Body(image).Do().Into(result)
	return
}

// ListImageRepositories returns a list of imagerepositories that match the selector.
func (c *Client) ListImageRepositories(ctx api.Context, selector labels.Selector) (result *imageapi.ImageRepositoryList, err error) {
	result = &imageapi.ImageRepositoryList{}
	err = c.Get().Path("imageRepositories").SelectorParam("labels", selector).Do().Into(result)
	return
}

// GetImageRepository returns information about a particular imagerepository and error if one occurs.
func (c *Client) GetImageRepository(ctx api.Context, id string) (result *imageapi.ImageRepository, err error) {
	result = &imageapi.ImageRepository{}
	err = c.Get().Path("imageRepositories").Path(id).Do().Into(result)
	return
}

// WatchImageRepositories returns a watch.Interface that watches the requested imagerepositories.
func (c *Client) WatchImageRepositories(ctx api.Context, field, label labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	return c.Get().
		Path("watch").
		Path("imageRepositories").
		UintParam("resourceVersion", resourceVersion).
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Watch()
}

// CreateImageRepository create a new imagerepository. Returns the server's representation of the imagerepository and error if one occurs.
func (c *Client) CreateImageRepository(ctx api.Context, repo *imageapi.ImageRepository) (result *imageapi.ImageRepository, err error) {
	result = &imageapi.ImageRepository{}
	err = c.Post().Path("imageRepositories").Body(repo).Do().Into(result)
	return
}

// UpdateImageRepository updates the imagerepository on the server. Returns the server's representation of the imagerepository and error if one occurs.
func (c *Client) UpdateImageRepository(ctx api.Context, repo *imageapi.ImageRepository) (result *imageapi.ImageRepository, err error) {
	result = &imageapi.ImageRepository{}
	err = c.Put().Path("imageRepositories").Path(repo.ID).Body(repo).Do().Into(result)
	return
}

// CreateImageRepositoryMapping create a new imagerepository mapping on the server. Returns error if one occurs.
func (c *Client) CreateImageRepositoryMapping(ctx api.Context, mapping *imageapi.ImageRepositoryMapping) error {
	return c.Post().Path("imageRepositoryMappings").Body(mapping).Do().Error()
}

// ListDeploymentConfigs takes a selector, and returns the list of deploymentConfigs that match that selector
func (c *Client) ListDeploymentConfigs(ctx api.Context, selector labels.Selector) (result *deployapi.DeploymentConfigList, err error) {
	result = &deployapi.DeploymentConfigList{}
	err = c.Get().Path("deploymentConfigs").SelectorParam("labels", selector).Do().Into(result)
	return
}

func (c *Client) WatchDeploymentConfigs(ctx api.Context, field, label labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	return c.Get().
		Path("watch").
		Path("deploymentConfigs").
		UintParam("resourceVersion", resourceVersion).
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Watch()
}

// GetDeploymentConfig returns information about a particular deploymentConfig
func (c *Client) GetDeploymentConfig(ctx api.Context, id string) (result *deployapi.DeploymentConfig, err error) {
	result = &deployapi.DeploymentConfig{}
	err = c.Get().Path("deploymentConfigs").Path(id).Do().Into(result)
	return
}

// CreateDeploymentConfig creates a new deploymentConfig
func (c *Client) CreateDeploymentConfig(ctx api.Context, deploymentConfig *deployapi.DeploymentConfig) (result *deployapi.DeploymentConfig, err error) {
	result = &deployapi.DeploymentConfig{}
	err = c.Post().Path("deploymentConfigs").Body(deploymentConfig).Do().Into(result)
	return
}

// UpdateDeploymentConfig updates an existing deploymentConfig
func (c *Client) UpdateDeploymentConfig(ctx api.Context, deploymentConfig *deployapi.DeploymentConfig) (result *deployapi.DeploymentConfig, err error) {
	result = &deployapi.DeploymentConfig{}
	err = c.Put().Path("deploymentConfigs").Path(deploymentConfig.ID).Body(deploymentConfig).Do().Into(result)
	return
}

// DeleteDeploymentConfig deletes an existing deploymentConfig.
func (c *Client) DeleteDeploymentConfig(ctx api.Context, id string) error {
	return c.Delete().Path("deploymentConfigs").Path(id).Do().Error()
}

// GenerateDeploymentConfig generates a new deploymentConfig for the given ID.
func (c *Client) GenerateDeploymentConfig(ctx api.Context, id string) (result *deployapi.DeploymentConfig, err error) {
	result = &deployapi.DeploymentConfig{}
	err = c.Get().Path("generateDeploymentConfigs").Path(id).Do().Into(result)
	return
}

// ListDeployments takes a selector, and returns the list of deployments that match that selector
func (c *Client) ListDeployments(ctx api.Context, selector labels.Selector) (result *deployapi.DeploymentList, err error) {
	result = &deployapi.DeploymentList{}
	err = c.Get().Path("deployments").SelectorParam("labels", selector).Do().Into(result)
	return
}

// GetDeployment returns information about a particular deployment
func (c *Client) GetDeployment(ctx api.Context, id string) (result *deployapi.Deployment, err error) {
	result = &deployapi.Deployment{}
	err = c.Get().Path("deployments").Path(id).Do().Into(result)
	return
}

// CreateDeployment creates a new deployment
func (c *Client) CreateDeployment(ctx api.Context, deployment *deployapi.Deployment) (result *deployapi.Deployment, err error) {
	result = &deployapi.Deployment{}
	err = c.Post().Path("deployments").Body(deployment).Do().Into(result)
	return
}

// UpdateDeployment updates an existing deployment
func (c *Client) UpdateDeployment(ctx api.Context, deployment *deployapi.Deployment) (result *deployapi.Deployment, err error) {
	result = &deployapi.Deployment{}
	err = c.Put().Path("deployments").Path(deployment.ID).Body(deployment).Do().Into(result)
	return
}

// DeleteDeployment deletes an existing replication deployment.
func (c *Client) DeleteDeployment(ctx api.Context, id string) error {
	return c.Delete().Path("deployments").Path(id).Do().Error()
}

// WatchDeployments returns a watch.Interface that watches the requested deployments.
func (c *Client) WatchDeployments(ctx api.Context, field, label labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	return c.Get().
		Path("watch").
		Path("deployments").
		UintParam("resourceVersion", resourceVersion).
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Watch()
}

// ListRoutes takes a selector, and returns the list of routes that match that selector
func (c *Client) ListRoutes(ctx api.Context, selector labels.Selector) (result *routeapi.RouteList, err error) {
	result = &routeapi.RouteList{}
	err = c.Get().Path("routes").SelectorParam("labels", selector).Do().Into(result)
	return
}

// GetRoute takes the name of the route, and returns the corresponding Route object, and an error if it occurs
func (c *Client) GetRoute(ctx api.Context, id string) (result *routeapi.Route, err error) {
	result = &routeapi.Route{}
	err = c.Get().Path("routes").Path(id).Do().Into(result)
	return
}

// DeleteRoute takes the name of the route, and returns an error if one occurs
func (c *Client) DeleteRoute(ctx api.Context, id string) error {
	return c.Delete().Path("routes").Path(id).Do().Error()
}

// CreateRoute takes the representation of a route.  Returns the server's representation of the route, and an error, if it occurs
func (c *Client) CreateRoute(ctx api.Context, route *routeapi.Route) (result *routeapi.Route, err error) {
	result = &routeapi.Route{}
	err = c.Post().Path("routes").Body(route).Do().Into(result)
	return
}

// UpdateRoute takes the representation of a route to update.  Returns the server's representation of the route, and an error, if it occurs
func (c *Client) UpdateRoute(ctx api.Context, route *routeapi.Route) (result *routeapi.Route, err error) {
	result = &routeapi.Route{}
	err = c.Put().Path("routes").Path(route.ID).Body(route).Do().Into(result)
	return
}

// WatchRoutes returns a watch.Interface that watches the requested routes.
func (c *Client) WatchRoutes(ctx api.Context, label, field labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	return c.Get().
		Path("watch").
		Path("routes").
		UintParam("resourceVersion", resourceVersion).
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Watch()
}
