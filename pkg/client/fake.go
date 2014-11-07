package client

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
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
	Ctx    kapi.Context
}

// Fake implements Interface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the method you want to test easier.
type Fake struct {
	// Fake by default keeps a simple list of the methods that have been called.
	Actions []FakeAction
}

func (c *Fake) CreateBuild(ctx kapi.Context, build *buildapi.Build) (*buildapi.Build, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-build", Ctx: ctx, Value: build})
	return &buildapi.Build{}, nil
}

func (c *Fake) ListBuilds(ctx kapi.Context, selector labels.Selector) (*buildapi.BuildList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-builds", Ctx: ctx})
	return &buildapi.BuildList{}, nil
}

func (c *Fake) GetBuild(ctx kapi.Context, id string) (*buildapi.Build, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-build"})
	return &buildapi.Build{}, nil
}

func (c *Fake) UpdateBuild(ctx kapi.Context, build *buildapi.Build) (*buildapi.Build, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-build", Ctx: ctx})
	return &buildapi.Build{}, nil
}

func (c *Fake) DeleteBuild(ctx kapi.Context, id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-build", Ctx: ctx, Value: id})
	return nil
}

func (c *Fake) WatchBuilds(ctx kapi.Context, field, label labels.Selector, resourceVersion string) (watch.Interface, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "watch-builds"})
	return nil, nil
}

func (c *Fake) CreateBuildConfig(ctx kapi.Context, config *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-buildconfig", Ctx: ctx})
	return &buildapi.BuildConfig{}, nil
}

func (c *Fake) ListBuildConfigs(ctx kapi.Context, selector labels.Selector) (*buildapi.BuildConfigList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-buildconfig", Ctx: ctx})
	return &buildapi.BuildConfigList{}, nil
}

func (c *Fake) GetBuildConfig(ctx kapi.Context, id string) (*buildapi.BuildConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-buildconfig", Ctx: ctx, Value: id})
	return &buildapi.BuildConfig{}, nil
}

func (c *Fake) UpdateBuildConfig(ctx kapi.Context, config *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-buildconfig", Ctx: ctx})
	return &buildapi.BuildConfig{}, nil
}

func (c *Fake) DeleteBuildConfig(ctx kapi.Context, id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-buildconfig", Ctx: ctx, Value: id})
	return nil
}

func (c *Fake) WatchDeploymentConfigs(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "watch-deploymentconfig"})
	return nil, nil
}

func (c *Fake) ListImages(ctx kapi.Context, selector labels.Selector) (*imageapi.ImageList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-images"})
	return &imageapi.ImageList{}, nil
}

func (c *Fake) GetImage(ctx kapi.Context, id string) (*imageapi.Image, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-image", Ctx: ctx, Value: id})
	return &imageapi.Image{}, nil
}

func (c *Fake) CreateImage(ctx kapi.Context, image *imageapi.Image) (*imageapi.Image, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-image", Ctx: ctx})
	return &imageapi.Image{}, nil
}

func (c *Fake) ListImageRepositories(ctx kapi.Context, selector labels.Selector) (*imageapi.ImageRepositoryList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-imagerepositries", Ctx: ctx})
	return &imageapi.ImageRepositoryList{}, nil
}

func (c *Fake) GetImageRepository(ctx kapi.Context, id string) (*imageapi.ImageRepository, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-imagerepository", Ctx: ctx, Value: id})
	return &imageapi.ImageRepository{}, nil
}

func (c *Fake) WatchImageRepositories(ctx kapi.Context, field, label labels.Selector, resourceVersion string) (watch.Interface, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "watch-imagerepositories"})
	return nil, nil
}

func (c *Fake) CreateImageRepository(ctx kapi.Context, repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-imagerepository", Ctx: ctx})
	return &imageapi.ImageRepository{}, nil
}

func (c *Fake) UpdateImageRepository(ctx kapi.Context, repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-imagerepository", Ctx: ctx})
	return &imageapi.ImageRepository{}, nil
}

func (c *Fake) CreateImageRepositoryMapping(ctx kapi.Context, mapping *imageapi.ImageRepositoryMapping) error {
	c.Actions = append(c.Actions, FakeAction{Action: "create-imagerepository-mapping", Ctx: ctx})
	return nil
}

func (c *Fake) ListDeploymentConfigs(ctx kapi.Context, label, field labels.Selector) (*deployapi.DeploymentConfigList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-deploymentconfig", Ctx: ctx})
	return &deployapi.DeploymentConfigList{}, nil
}

func (c *Fake) GetDeploymentConfig(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-deploymentconfig", Ctx: ctx})
	return &deployapi.DeploymentConfig{}, nil
}

func (c *Fake) CreateDeploymentConfig(ctx kapi.Context, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-deploymentconfig", Ctx: ctx})
	return &deployapi.DeploymentConfig{}, nil
}

func (c *Fake) UpdateDeploymentConfig(ctx kapi.Context, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-deploymentconfig", Ctx: ctx})
	return &deployapi.DeploymentConfig{}, nil
}

func (c *Fake) DeleteDeploymentConfig(ctx kapi.Context, id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-deploymentconfig", Ctx: ctx})
	return nil
}

func (c *Fake) GenerateDeploymentConfig(ctx kapi.Context, id string) (*deployapi.DeploymentConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "generate-deploymentconfig"})
	return nil, nil
}

func (c *Fake) ListDeployments(ctx kapi.Context, label, field labels.Selector) (*deployapi.DeploymentList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-deployment"})
	return &deployapi.DeploymentList{}, nil
}

func (c *Fake) GetDeployment(ctx kapi.Context, id string) (*deployapi.Deployment, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-deployment", Ctx: ctx})
	return &deployapi.Deployment{}, nil
}

func (c *Fake) CreateDeployment(ctx kapi.Context, deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-deployment", Ctx: ctx})
	return &deployapi.Deployment{}, nil
}

func (c *Fake) UpdateDeployment(ctx kapi.Context, deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-deployment", Value: deployment})
	return &deployapi.Deployment{}, nil
}

func (c *Fake) DeleteDeployment(ctx kapi.Context, id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-deployment", Ctx: ctx})
	return nil
}

func (c *Fake) WatchDeployments(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "watch-deployments"})
	return nil, nil
}

func (c *Fake) ListRoutes(ctx kapi.Context, selector labels.Selector) (*routeapi.RouteList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-routes"})
	return &routeapi.RouteList{}, nil
}

func (c *Fake) GetRoute(ctx kapi.Context, id string) (*routeapi.Route, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-route", Ctx: ctx})
	return &routeapi.Route{}, nil
}

func (c *Fake) CreateRoute(ctx kapi.Context, route *routeapi.Route) (*routeapi.Route, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-route", Ctx: ctx})
	return &routeapi.Route{}, nil
}

func (c *Fake) UpdateRoute(ctx kapi.Context, route *routeapi.Route) (*routeapi.Route, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-route", Ctx: ctx})
	return &routeapi.Route{}, nil
}

func (c *Fake) DeleteRoute(ctx kapi.Context, id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-route", Ctx: ctx})
	return nil
}

func (c *Fake) WatchRoutes(ctx kapi.Context, field, label labels.Selector, resourceVersion string) (watch.Interface, error) {
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
