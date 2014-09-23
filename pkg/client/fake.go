package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
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

func (c *Fake) CreateBuild(build *buildapi.Build) (*buildapi.Build, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-build"})
	return &buildapi.Build{}, nil
}

func (c *Fake) ListBuilds(selector labels.Selector) (*buildapi.BuildList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-builds"})
	return &buildapi.BuildList{}, nil
}

func (c *Fake) UpdateBuild(build *buildapi.Build) (*buildapi.Build, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-build"})
	return &buildapi.Build{}, nil
}

func (c *Fake) DeleteBuild(id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-build", Value: id})
	return nil
}

func (c *Fake) CreateBuildConfig(config *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-buildconfig"})
	return &buildapi.BuildConfig{}, nil
}

func (c *Fake) ListBuildConfigs(selector labels.Selector) (*buildapi.BuildConfigList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-buildconfig"})
	return &buildapi.BuildConfigList{}, nil
}

func (c *Fake) GetBuildConfig(id string) (*buildapi.BuildConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-buildconfig", Value: id})
	return &buildapi.BuildConfig{}, nil
}

func (c *Fake) UpdateBuildConfig(config *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-buildconfig"})
	return &buildapi.BuildConfig{}, nil
}

func (c *Fake) DeleteBuildConfig(id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-buildconfig", Value: id})
	return nil
}

func (c *Fake) ListImages(selector labels.Selector) (*imageapi.ImageList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-images"})
	return &imageapi.ImageList{}, nil
}

func (c *Fake) GetImage(id string) (*imageapi.Image, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-image", Value: id})
	return &imageapi.Image{}, nil
}

func (c *Fake) CreateImage(image *imageapi.Image) (*imageapi.Image, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-image"})
	return &imageapi.Image{}, nil
}

func (c *Fake) ListImageRepositories(selector labels.Selector) (*imageapi.ImageRepositoryList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-imagerepositries"})
	return &imageapi.ImageRepositoryList{}, nil
}

func (c *Fake) GetImageRepository(id string) (*imageapi.ImageRepository, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-imagerepository", Value: id})
	return &imageapi.ImageRepository{}, nil
}

func (c *Fake) WatchImageRepositories(field, label labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "watch-imagerepositories"})
	return nil, nil
}

func (c *Fake) CreateImageRepository(repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-imagerepository"})
	return &imageapi.ImageRepository{}, nil
}

func (c *Fake) UpdateImageRepository(repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-imagerepository"})
	return &imageapi.ImageRepository{}, nil
}

func (c *Fake) CreateImageRepositoryMapping(mapping *imageapi.ImageRepositoryMapping) error {
	c.Actions = append(c.Actions, FakeAction{Action: "create-imagerepository-mapping"})
	return nil
}

func (c *Fake) ListDeploymentConfigs(selector labels.Selector) (*deployapi.DeploymentConfigList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-deploymentconfig"})
	return &deployapi.DeploymentConfigList{}, nil
}

func (c *Fake) GetDeploymentConfig(id string) (*deployapi.DeploymentConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-deploymentconfig"})
	return &deployapi.DeploymentConfig{}, nil
}

func (c *Fake) CreateDeploymentConfig(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-deploymentconfig"})
	return &deployapi.DeploymentConfig{}, nil
}

func (c *Fake) UpdateDeploymentConfig(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-deploymentconfig"})
	return &deployapi.DeploymentConfig{}, nil
}

func (c *Fake) DeleteDeploymentConfig(id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-deploymentconfig"})
	return nil
}

func (c *Fake) ListDeployments(selector labels.Selector) (*deployapi.DeploymentList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-deployment"})
	return &deployapi.DeploymentList{}, nil
}

func (c *Fake) GetDeployment(id string) (*deployapi.Deployment, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-deployment"})
	return &deployapi.Deployment{}, nil
}

func (c *Fake) CreateDeployment(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-deployment"})
	return &deployapi.Deployment{}, nil
}

func (c *Fake) UpdateDeployment(deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-deployment"})
	return &deployapi.Deployment{}, nil
}

func (c *Fake) DeleteDeployment(id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-deployment"})
	return nil
}
