package client

import (
	"fmt"

	"github.com/openshift/origin/pkg/api/latest"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/extensions"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
)

type delegatingScaleInterface struct {
	dcs    DeploymentConfigInterface
	scales kclient.ScaleInterface
}

type delegatingScaleNamespacer struct {
	dcNS    DeploymentConfigsNamespacer
	scaleNS kclient.ScaleNamespacer
}

func (c *delegatingScaleNamespacer) Scales(namespace string) kclient.ScaleInterface {
	return &delegatingScaleInterface{
		dcs:    c.dcNS.DeploymentConfigs(namespace),
		scales: c.scaleNS.Scales(namespace),
	}
}

func NewDelegatingScaleNamespacer(dcNamespacer DeploymentConfigsNamespacer, sNamespacer kclient.ScaleNamespacer) kclient.ScaleNamespacer {
	return &delegatingScaleNamespacer{
		dcNS:    dcNamespacer,
		scaleNS: sNamespacer,
	}
}

// Get takes the reference to scale subresource and returns the subresource or error, if one occurs.
func (c *delegatingScaleInterface) Get(kind string, name string) (result *extensions.Scale, err error) {
	switch {
	case kind == "DeploymentConfig":
		return c.dcs.GetScale(name)
	case latest.OriginKind(kind, ""):
		return nil, errors.NewBadRequest(fmt.Sprintf("Kind %s has no Scale subresource", kind))
	default:
		return c.scales.Get(kind, name)
	}
}

// Update takes a scale subresource object, updates the stored version to match it, and
// returns the subresource or error, if one occurs.
func (c *delegatingScaleInterface) Update(kind string, scale *extensions.Scale) (result *extensions.Scale, err error) {
	switch {
	case kind == "DeploymentConfig":
		return c.dcs.UpdateScale(scale)
	case latest.OriginKind(kind, ""):
		return nil, errors.NewBadRequest(fmt.Sprintf("Kind %s has no Scale subresource", kind))
	default:
		return c.scales.Update(kind, scale)
	}
}
