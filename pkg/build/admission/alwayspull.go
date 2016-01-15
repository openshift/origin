package admission

import (
	"fmt"
	"io"

	"k8s.io/kubernetes/pkg/admission"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

func init() {
	admission.RegisterPlugin("AlwaysPullBuildImages", func(c kclient.Interface, config io.Reader) (admission.Interface, error) {
		return NewAlwaysPullBuildImages(), nil
	})
}

type alwaysPull struct {
	*admission.Handler
}

// NewAlwaysPullBuildImages returns an admission controller for builds that sets ForcePull to true
// regardless of the user's desired value. This is to ensure that a user has access to use the
// image, since once an image has been pulled to a given node, access to that image is not
// restricted or verified in any way (yet).
func NewAlwaysPullBuildImages() admission.Interface {
	return &alwaysPull{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}
}

func (a *alwaysPull) Admit(attr admission.Attributes) error {
	if resource := attr.GetResource(); resource != buildsResource && resource != buildConfigsResource {
		return nil
	}
	switch obj := attr.GetObject().(type) {
	case *buildapi.Build:
		setForcePull(obj.Spec.Strategy)
		return nil
	case *buildapi.BuildConfig:
		setForcePull(obj.Spec.Strategy)
		return nil
	default:
		return admission.NewForbidden(attr, fmt.Errorf("unrecognized request object %#v", obj))
	}
}

func setForcePull(strategy buildapi.BuildStrategy) {
	switch {
	case strategy.DockerStrategy != nil:
		strategy.DockerStrategy.ForcePull = true
	case strategy.CustomStrategy != nil:
		strategy.CustomStrategy.ForcePull = true
	case strategy.SourceStrategy != nil:
		strategy.SourceStrategy.ForcePull = true
	}
}
