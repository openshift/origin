package fake

import (
	v1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeImageStreamMappings implements ImageStreamMappingInterface
type FakeImageStreamMappings struct {
	Fake *FakeImageV1
	ns   string
}

var imagestreammappingsResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "imagestreammappings"}

func (c *FakeImageStreamMappings) Create(imageStreamMapping *v1.ImageStreamMapping) (result *v1.ImageStreamMapping, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(imagestreammappingsResource, c.ns, imageStreamMapping), &v1.ImageStreamMapping{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ImageStreamMapping), err
}
