package admission

import (
	"io"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api/errors"
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
	if len(attr.GetSubresource()) != 0 || attr.GetResource() != buildsResource {
		return nil
	}

	build, ok := attr.GetObject().(*buildapi.Build)
	if !ok {
		return errors.NewBadRequest("Resource was marked with kind Build but was unable to be converted")
	}

	setForcePull(build.Spec.Strategy)

	return nil
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
