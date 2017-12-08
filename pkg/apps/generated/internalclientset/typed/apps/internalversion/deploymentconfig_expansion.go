package internalversion

import (
	v1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
)

type DeploymentConfigExpansion interface {
	UpdateScale(deploymentConfigName string, scale *v1beta1.Scale) (result *v1beta1.Scale, err error)
}

var scaleCodec = legacyscheme.Codecs.LegacyCodec(v1beta1.SchemeGroupVersion)

// UpdateScale takes the top resource name and the representation of a scale and updates it. Returns the server's representation of the scale, and an error, if there is any.
func (c *deploymentConfigs) UpdateScale(deploymentConfigName string, scale *v1beta1.Scale) (result *v1beta1.Scale, err error) {
	// FIXME: make non-homogenous subresource GV client generation work
	data, err := runtime.Encode(scaleCodec, scale)
	if err != nil {
		return nil, err
	}

	result = &v1beta1.Scale{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("deploymentconfigs").
		Name(deploymentConfigName).
		SubResource("scale").
		Body(data).
		Do().
		Into(result)
	return
}
