package fake

import (
	image "github.com/openshift/origin/pkg/image/apis/image"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeImageStreamImports implements ImageStreamImportInterface
type FakeImageStreamImports struct {
	Fake *FakeImage
	ns   string
}

var imagestreamimportsResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "", Resource: "imagestreamimports"}

var imagestreamimportsKind = schema.GroupVersionKind{Group: "image.openshift.io", Version: "", Kind: "ImageStreamImport"}

// Create takes the representation of a imageStreamImport and creates it.  Returns the server's representation of the imageStreamImport, and an error, if there is any.
func (c *FakeImageStreamImports) Create(imageStreamImport *image.ImageStreamImport) (result *image.ImageStreamImport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(imagestreamimportsResource, c.ns, imageStreamImport), &image.ImageStreamImport{})

	if obj == nil {
		return nil, err
	}
	return obj.(*image.ImageStreamImport), err
}
