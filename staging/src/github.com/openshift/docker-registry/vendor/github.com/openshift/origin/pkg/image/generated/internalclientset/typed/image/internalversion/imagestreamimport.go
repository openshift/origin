package internalversion

import (
	image "github.com/openshift/origin/pkg/image/apis/image"
	rest "k8s.io/client-go/rest"
)

// ImageStreamImportsGetter has a method to return a ImageStreamImportInterface.
// A group's client should implement this interface.
type ImageStreamImportsGetter interface {
	ImageStreamImports(namespace string) ImageStreamImportInterface
}

// ImageStreamImportInterface has methods to work with ImageStreamImport resources.
type ImageStreamImportInterface interface {
	Create(*image.ImageStreamImport) (*image.ImageStreamImport, error)
	ImageStreamImportExpansion
}

// imageStreamImports implements ImageStreamImportInterface
type imageStreamImports struct {
	client rest.Interface
	ns     string
}

// newImageStreamImports returns a ImageStreamImports
func newImageStreamImports(c *ImageClient, namespace string) *imageStreamImports {
	return &imageStreamImports{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a imageStreamImport and creates it.  Returns the server's representation of the imageStreamImport, and an error, if there is any.
func (c *imageStreamImports) Create(imageStreamImport *image.ImageStreamImport) (result *image.ImageStreamImport, err error) {
	result = &image.ImageStreamImport{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("imagestreamimports").
		Body(imageStreamImport).
		Do().
		Into(result)
	return
}
