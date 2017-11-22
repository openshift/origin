package fake

import (
	v1 "github.com/openshift/api/image/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeImageSignatures implements ImageSignatureInterface
type FakeImageSignatures struct {
	Fake *FakeImageV1
}

var imagesignaturesResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "imagesignatures"}

var imagesignaturesKind = schema.GroupVersionKind{Group: "image.openshift.io", Version: "v1", Kind: "ImageSignature"}

// Create takes the representation of a imageSignature and creates it.  Returns the server's representation of the imageSignature, and an error, if there is any.
func (c *FakeImageSignatures) Create(imageSignature *v1.ImageSignature) (result *v1.ImageSignature, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(imagesignaturesResource, imageSignature), &v1.ImageSignature{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ImageSignature), err
}

// Delete takes name of the imageSignature and deletes it. Returns an error if one occurs.
func (c *FakeImageSignatures) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(imagesignaturesResource, name), &v1.ImageSignature{})
	return err
}
