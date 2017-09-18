package v1

import (
	v1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	rest "k8s.io/client-go/rest"
)

// ImageStreamMappingsGetter has a method to return a ImageStreamMappingInterface.
// A group's client should implement this interface.
type ImageStreamMappingsGetter interface {
	ImageStreamMappings(namespace string) ImageStreamMappingInterface
}

// ImageStreamMappingInterface has methods to work with ImageStreamMapping resources.
type ImageStreamMappingInterface interface {
	Create(*v1.ImageStreamMapping) (*v1.ImageStreamMapping, error)
	ImageStreamMappingExpansion
}

// imageStreamMappings implements ImageStreamMappingInterface
type imageStreamMappings struct {
	client rest.Interface
	ns     string
}

// newImageStreamMappings returns a ImageStreamMappings
func newImageStreamMappings(c *ImageV1Client, namespace string) *imageStreamMappings {
	return &imageStreamMappings{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a imageStreamMapping and creates it.  Returns the server's representation of the imageStreamMapping, and an error, if there is any.
func (c *imageStreamMappings) Create(imageStreamMapping *v1.ImageStreamMapping) (result *v1.ImageStreamMapping, err error) {
	result = &v1.ImageStreamMapping{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("imagestreammappings").
		Body(imageStreamMapping).
		Do().
		Into(result)
	return
}
