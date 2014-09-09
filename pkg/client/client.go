package client

import (
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	buildapi "github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/build/api/v1beta1"
	imageapi "github.com/openshift/origin/pkg/image/api"
	_ "github.com/openshift/origin/pkg/image/api/v1beta1"
)

// Interface exposes methods on OpenShift resources.
type Interface interface {
	BuildInterface
	ImageInterface
	ImageRepositoryInterface
	ImageRepositoryMappingInterface
}

// BuildInterface exposes methods on Build resources.
type BuildInterface interface {
	ListBuilds(selector labels.Selector) (buildapi.BuildList, error)
	UpdateBuild(buildapi.Build) (buildapi.Build, error)
}

// ImageInterface exposes methods on Image resources.
type ImageInterface interface {
	ListImages(selector labels.Selector) (imageapi.ImageList, error)
	GetImage(id string) (imageapi.Image, error)
	CreateImage(imageapi.Image) (imageapi.Image, error)
}

// ImageRepositoryInterface exposes methods on ImageRepository resources.
type ImageRepositoryInterface interface {
	ListImageRepositories(selector labels.Selector) (imageapi.ImageRepositoryList, error)
	GetImageRepository(id string) (imageapi.ImageRepository, error)
	WatchImageRepositories(field, label labels.Selector, resourceVersion uint64) (watch.Interface, error)
	CreateImageRepository(repo imageapi.ImageRepository) (imageapi.ImageRepository, error)
	UpdateImageRepository(repo imageapi.ImageRepository) (imageapi.ImageRepository, error)
}

// ImageRepositoryMappingInterface exposes methods on ImageRepositoryMapping resources.
type ImageRepositoryMappingInterface interface {
	CreateImageRepositoryMapping(mapping imageapi.ImageRepositoryMapping) error
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

// ListBuilds returns a list of builds.
func (c *Client) ListBuilds(selector labels.Selector) (result buildapi.BuildList, err error) {
	err = c.Get().Path("builds").SelectorParam("labels", selector).Do().Into(&result)
	return
}

// UpdateBuild updates an existing build.
func (c *Client) UpdateBuild(build buildapi.Build) (result buildapi.Build, err error) {
	err = c.Put().Path("builds").Path(build.ID).Body(build).Do().Into(&result)
	return
}

func (c *Client) ListImages(selector labels.Selector) (result imageapi.ImageList, err error) {
	err = c.Get().Path("images").SelectorParam("labels", selector).Do().Into(&result)
	return
}

func (c *Client) GetImage(id string) (result imageapi.Image, err error) {
	err = c.Get().Path("images").Path(id).Do().Into(&result)
	return
}

func (c *Client) CreateImage(image imageapi.Image) (result imageapi.Image, err error) {
	err = c.Post().Path("images").Body(image).Do().Into(&result)
	return
}

func (c *Client) ListImageRepositories(selector labels.Selector) (result imageapi.ImageRepositoryList, err error) {
	err = c.Get().Path("imageRepositories").SelectorParam("labels", selector).Do().Into(&result)
	return
}

func (c *Client) GetImageRepository(id string) (result imageapi.ImageRepository, err error) {
	err = c.Get().Path("imageRepositories").Path(id).Do().Into(&result)
	return
}

func (c *Client) WatchImageRepositories(field, label labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	return c.Get().
		Path("watch").
		Path("imageRepositories").
		UintParam("resourceVersion", resourceVersion).
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Watch()
}

func (c *Client) CreateImageRepository(repo imageapi.ImageRepository) (result imageapi.ImageRepository, err error) {
	err = c.Post().Path("imageRepositories").Body(repo).Do().Into(&result)
	return
}

func (c *Client) UpdateImageRepository(repo imageapi.ImageRepository) (result imageapi.ImageRepository, err error) {
	err = c.Put().Path("imageRepositories").Path(repo.ID).Body(repo).Do().Into(&result)
	return
}

func (c *Client) CreateImageRepositoryMapping(mapping imageapi.ImageRepositoryMapping) error {
	return c.Post().Path("imageRepositoryMappings").Body(mapping).Do().Error()
}
