package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	userapi "github.com/openshift/origin/pkg/user/api"
)

type FakeAction struct {
	Action string
	Value  interface{}
}

// Fake implements Interface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the method you want to test easier.
type Fake struct {
	// Fake by default keeps a simple list of the methods that have been called.
	Actions []FakeAction
}

func (c *Fake) CreateBuild(ctx api.Context, build *buildapi.Build) (*buildapi.Build, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-build"})
	return &buildapi.Build{}, nil
}

func (c *Fake) ListBuilds(ctx api.Context, selector labels.Selector) (*buildapi.BuildList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-builds"})
	return &buildapi.BuildList{}, nil
}

func (c *Fake) UpdateBuild(ctx api.Context, build *buildapi.Build) (*buildapi.Build, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-build"})
	return &buildapi.Build{}, nil
}

func (c *Fake) DeleteBuild(ctx api.Context, id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-build", Value: id})
	return nil
}

func (c *Fake) CreateBuildConfig(ctx api.Context, config *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-buildconfig"})
	return &buildapi.BuildConfig{}, nil
}

func (c *Fake) ListBuildConfigs(ctx api.Context, selector labels.Selector) (*buildapi.BuildConfigList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-buildconfig"})
	return &buildapi.BuildConfigList{}, nil
}

func (c *Fake) GetBuildConfig(ctx api.Context, id string) (*buildapi.BuildConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-buildconfig", Value: id})
	return &buildapi.BuildConfig{}, nil
}

func (c *Fake) UpdateBuildConfig(ctx api.Context, config *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-buildconfig"})
	return &buildapi.BuildConfig{}, nil
}

func (c *Fake) DeleteBuildConfig(ctx api.Context, id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-buildconfig", Value: id})
	return nil
}

func (c *Fake) WatchDeploymentConfigs(ctx api.Context, field, label labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "watch-deploymentconfig"})
	return nil, nil
}

func (c *Fake) ListImages(ctx api.Context, selector labels.Selector) (*imageapi.ImageList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-images"})
	return &imageapi.ImageList{}, nil
}

func (c *Fake) GetImage(ctx api.Context, id string) (*imageapi.Image, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-image", Value: id})
	return &imageapi.Image{}, nil
}

func (c *Fake) CreateImage(ctx api.Context, image *imageapi.Image) (*imageapi.Image, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-image"})
	return &imageapi.Image{}, nil
}

func (c *Fake) ListImageRepositories(ctx api.Context, labels labels.Selector) (*imageapi.ImageRepositoryList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-imagerepositories"})
	return &imageapi.ImageRepositoryList{}, nil
}

func (c *Fake) GetImageRepository(ctx api.Context, id string) (*imageapi.ImageRepository, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-imagerepository", Value: id})
	return &imageapi.ImageRepository{}, nil
}

func (c *Fake) WatchImageRepositories(ctx api.Context, field, label labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "watch-imagerepositories"})
	return nil, nil
}

func (c *Fake) CreateImageRepository(ctx api.Context, repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-imagerepository"})
	return &imageapi.ImageRepository{}, nil
}

func (c *Fake) UpdateImageRepository(ctx api.Context, repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-imagerepository"})
	return &imageapi.ImageRepository{}, nil
}

func (c *Fake) CreateImageRepositoryMapping(ctx api.Context, mapping *imageapi.ImageRepositoryMapping) error {
	c.Actions = append(c.Actions, FakeAction{Action: "create-imagerepository-mapping"})
	return nil
}

func (c *Fake) ListDeploymentConfigs(ctx api.Context, selector labels.Selector) (*deployapi.DeploymentConfigList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-deploymentconfig"})
	return &deployapi.DeploymentConfigList{}, nil
}

func (c *Fake) GetDeploymentConfig(ctx api.Context, id string) (*deployapi.DeploymentConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-deploymentconfig"})
	return &deployapi.DeploymentConfig{}, nil
}

func (c *Fake) CreateDeploymentConfig(ctx api.Context, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-deploymentconfig"})
	return &deployapi.DeploymentConfig{}, nil
}

func (c *Fake) UpdateDeploymentConfig(ctx api.Context, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-deploymentconfig"})
	return &deployapi.DeploymentConfig{}, nil
}

func (c *Fake) DeleteDeploymentConfig(ctx api.Context, id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-deploymentconfig"})
	return nil
}

func (c *Fake) GenerateDeploymentConfig(ctx api.Context, id string) (*deployapi.DeploymentConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "generate-deploymentconfig"})
	return nil, nil
}

func (c *Fake) ListDeployments(ctx api.Context, selector labels.Selector) (*deployapi.DeploymentList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-deployment"})
	return &deployapi.DeploymentList{}, nil
}

func (c *Fake) GetDeployment(ctx api.Context, id string) (*deployapi.Deployment, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-deployment"})
	return &deployapi.Deployment{}, nil
}

func (c *Fake) CreateDeployment(ctx api.Context, deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-deployment"})
	return &deployapi.Deployment{}, nil
}

func (c *Fake) UpdateDeployment(ctx api.Context, deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-deployment", Value: deployment})
	return &deployapi.Deployment{}, nil
}

func (c *Fake) DeleteDeployment(ctx api.Context, id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-deployment"})
	return nil
}

func (c *Fake) WatchDeployments(ctx api.Context, field, label labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "watch-deployments"})
	return nil, nil
}

func (c *Fake) ListRoutes(ctx api.Context, selector labels.Selector) (*routeapi.RouteList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-routes"})
	return &routeapi.RouteList{}, nil
}

func (c *Fake) GetRoute(ctx api.Context, id string) (*routeapi.Route, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-route"})
	return &routeapi.Route{}, nil
}

func (c *Fake) CreateRoute(ctx api.Context, route *routeapi.Route) (*routeapi.Route, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-route"})
	return &routeapi.Route{}, nil
}

func (c *Fake) UpdateRoute(ctx api.Context, route *routeapi.Route) (*routeapi.Route, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-route"})
	return &routeapi.Route{}, nil
}

func (c *Fake) DeleteRoute(ctx api.Context, id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-route"})
	return nil
}

func (c *Fake) WatchRoutes(ctx api.Context, field, label labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "watch-routes"})
	return nil, nil
}

func (c *Fake) GetUser(id string) (*userapi.User, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-user", Value: id})
	return &userapi.User{}, nil
}

func (c *Fake) CreateOrUpdateUserIdentityMapping(mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, bool, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "createorupdate-useridentitymapping"})
	return nil, false, nil
}
