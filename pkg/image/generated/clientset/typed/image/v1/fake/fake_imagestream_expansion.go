package fake

import (
	"io"

	image_v1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

var imagestreamtagsinstantiateResource = schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "imagestreamtags/instantiate"}

func (c *FakeImageStreams) Instantiate(imageStream *image_v1.ImageStreamTagInstantiate, r io.Reader) (result *image_v1.ImageStreamTag, err error) {
	obj, err := c.Fake.Invokes(testing.NewCreateAction(imagestreamtagsinstantiateResource, c.ns, imageStream), &image_v1.ImageStreamTag{})
	if obj == nil {
		return nil, err
	}
	return obj.(*image_v1.ImageStreamTag), err
}
