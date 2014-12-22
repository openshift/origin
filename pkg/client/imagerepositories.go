package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ImageRepositoriesNamespacer has methods to work with ImageRepository resources in a namespace
type ImageRepositoriesNamespacer interface {
	ImageRepositories(namespace string) ImageRepositoryInterface
}

// ImageRepositoryInterface exposes methods on ImageRepository resources.
type ImageRepositoryInterface interface {
	List(label, field labels.Selector) (*imageapi.ImageRepositoryList, error)
	Get(name string) (*imageapi.ImageRepository, error)
	Create(repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error)
	Update(repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error)
	Watch(label, field labels.Selector, resourceVersion string) (watch.Interface, error)
}

// imageRepositories implements ImageRepositoriesNamespacer interface
type imageRepositories struct {
	r  *Client
	ns string
}

// newImageRepositories returns an imageRepositories
func newImageRepositories(c *Client, namespace string) *imageRepositories {
	return &imageRepositories{
		r:  c,
		ns: namespace,
	}
}

// List returns a list of imagerepositories that match the label and fiels selectors.
func (c *imageRepositories) List(label, field labels.Selector) (result *imageapi.ImageRepositoryList, err error) {
	result = &imageapi.ImageRepositoryList{}
	err = c.r.Get().
		Namespace(c.ns).
		Path("imageRepositories").
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Do().
		Into(result)
	return
}

// Get returns information about a particular imagerepository and error if one occurs.
func (c *imageRepositories) Get(name string) (result *imageapi.ImageRepository, err error) {
	result = &imageapi.ImageRepository{}
	err = c.r.Get().Namespace(c.ns).Path("imageRepositories").Path(name).Do().Into(result)
	return
}

// Create create a new imagerepository. Returns the server's representation of the imagerepository and error if one occurs.
func (c *imageRepositories) Create(repo *imageapi.ImageRepository) (result *imageapi.ImageRepository, err error) {
	result = &imageapi.ImageRepository{}
	err = c.r.Post().Namespace(c.ns).Path("imageRepositories").Body(repo).Do().Into(result)
	return
}

// Update updates the imagerepository on the server. Returns the server's representation of the imagerepository and error if one occurs.
func (c *imageRepositories) Update(repo *imageapi.ImageRepository) (result *imageapi.ImageRepository, err error) {
	result = &imageapi.ImageRepository{}
	err = c.r.Put().Namespace(c.ns).Path("imageRepositories").Path(repo.Name).Body(repo).Do().Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested imagerepositories.
func (c *imageRepositories) Watch(label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	return c.r.Get().
		Path("watch").
		Namespace(c.ns).
		Path("imageRepositories").
		Param("resourceVersion", resourceVersion).
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Watch()
}
