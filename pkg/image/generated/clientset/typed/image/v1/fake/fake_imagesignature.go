package fake

import (
	v1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeImageSignatures implements ImageSignatureInterface
type FakeImageSignatures struct {
	Fake *FakeImageV1
	ns   string
}

var imagesignaturesResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "imagesignatures"}

func (c *FakeImageSignatures) Create(imageSignature *v1.ImageSignature) (result *v1.ImageSignature, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(imagesignaturesResource, c.ns, imageSignature), &v1.ImageSignature{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ImageSignature), err
}

func (c *FakeImageSignatures) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(imagesignaturesResource, c.ns, name), &v1.ImageSignature{})

	return err
}
