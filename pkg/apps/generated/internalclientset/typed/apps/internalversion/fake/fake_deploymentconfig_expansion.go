package fake

import (
	v1beta1 "k8s.io/api/extensions/v1beta1"
	testing "k8s.io/client-go/testing"
)

// UpdateScale takes the representation of a scale and updates it. Returns the server's representation of the scale, and an error, if t
func (c *FakeDeploymentConfigs) UpdateScale(deploymentConfigName string, scale *v1beta1.Scale) (result *v1beta1.Scale, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(deploymentconfigsResource, "scale", c.ns, scale), &v1beta1.Scale{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.Scale), err
}
