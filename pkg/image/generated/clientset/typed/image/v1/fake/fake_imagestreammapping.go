package fake

import (
	v1 "github.com/openshift/api/image/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeImageStreamMappings implements ImageStreamMappingInterface
type FakeImageStreamMappings struct {
	Fake *FakeImageV1
	ns   string
}

var imagestreammappingsResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "imagestreammappings"}

var imagestreammappingsKind = schema.GroupVersionKind{Group: "image.openshift.io", Version: "v1", Kind: "ImageStreamMapping"}

// Create takes the representation of a imageStreamMapping and creates it.  Returns the server's representation of the status, and an error, if there is any.
func (c *FakeImageStreamMappings) Create(imageStreamMapping *v1.ImageStreamMapping) (result *meta_v1.Status, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(imagestreammappingsResource, c.ns, imageStreamMapping), &meta_v1.Status{})

	if obj == nil {
		return nil, err
	}
	return obj.(*meta_v1.Status), err
}
