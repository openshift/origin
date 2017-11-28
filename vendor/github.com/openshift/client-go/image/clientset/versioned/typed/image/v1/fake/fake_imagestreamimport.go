package fake

import (
	v1 "github.com/openshift/api/image/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeImageStreamImports implements ImageStreamImportInterface
type FakeImageStreamImports struct {
	Fake *FakeImageV1
	ns   string
}

var imagestreamimportsResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "imagestreamimports"}

var imagestreamimportsKind = schema.GroupVersionKind{Group: "image.openshift.io", Version: "v1", Kind: "ImageStreamImport"}

// Create takes the representation of a imageStreamImport and creates it.  Returns the server's representation of the imageStreamImport, and an error, if there is any.
func (c *FakeImageStreamImports) Create(imageStreamImport *v1.ImageStreamImport) (result *v1.ImageStreamImport, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(imagestreamimportsResource, c.ns, imageStreamImport), &v1.ImageStreamImport{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ImageStreamImport), err
}
