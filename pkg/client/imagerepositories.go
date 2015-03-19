package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
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
	List(label labels.Selector, field fields.Selector) (*imageapi.ImageRepositoryList, error)
	Get(name string) (*imageapi.ImageRepository, error)
	Create(repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error)
	Update(repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error)
	Delete(name string) error
	Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
}

// ImageRepositoryNamespaceGetter exposes methods to get ImageRepositories by Namespace
type ImageRepositoryNamespaceGetter interface {
	GetByNamespace(namespace, name string) (*imageapi.ImageRepository, error)
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

// List returns a list of imagerepositories that match the label and field selectors.
func (c *imageRepositories) List(label labels.Selector, field fields.Selector) (result *imageapi.ImageRepositoryList, err error) {
	result = &imageapi.ImageRepositoryList{}
	err = c.r.Get().
		Namespace(c.ns).
		Resource("imageRepositories").
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Do().
		Into(result)
	return
}

// Get returns information about a particular imagerepository and error if one occurs.
func (c *imageRepositories) Get(name string) (result *imageapi.ImageRepository, err error) {
	result = &imageapi.ImageRepository{}
	err = c.r.Get().Namespace(c.ns).Resource("imageRepositories").Name(name).Do().Into(result)
	return
}

// GetByNamespace returns information about a particular imagerepository in a particular namespace and error if one occurs.
func (c *imageRepositories) GetByNamespace(namespace, name string) (result *imageapi.ImageRepository, err error) {
	result = &imageapi.ImageRepository{}
	c.r.Get().Namespace(namespace).Resource("imageRepositories").Name(name).Do().Into(result)
	return
}

// Create create a new imagerepository. Returns the server's representation of the imagerepository and error if one occurs.
func (c *imageRepositories) Create(repo *imageapi.ImageRepository) (result *imageapi.ImageRepository, err error) {
	result = &imageapi.ImageRepository{}
	err = c.r.Post().Namespace(c.ns).Resource("imageRepositories").Body(repo).Do().Into(result)
	return
}

// Update updates the imagerepository on the server. Returns the server's representation of the imagerepository and error if one occurs.
func (c *imageRepositories) Update(repo *imageapi.ImageRepository) (result *imageapi.ImageRepository, err error) {
	result = &imageapi.ImageRepository{}
	err = c.r.Put().Namespace(c.ns).Resource("imageRepositories").Name(repo.Name).Body(repo).Do().Into(result)
	return
}

// Delete deletes an image repository, returns error if one occurs.
func (c *imageRepositories) Delete(name string) (err error) {
	err = c.r.Delete().Namespace(c.ns).Resource("imageRepositories").Name(name).Do().Error()
	return
}

// Watch returns a watch.Interface that watches the requested imagerepositories.
func (c *imageRepositories) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return c.r.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("imageRepositories").
		Param("resourceVersion", resourceVersion).
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Watch()
}
