package admission

import (
	"fmt"
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

const buildRequestsResource = "buildrequests"

func (a *alwaysPull) Admit(attr admission.Attributes) error {
	if len(attr.GetSubresource()) != 0 || (attr.GetResource() != buildsResource && attr.GetResource() != buildRequestsResource) {
		return nil
	}

	switch obj := attr.GetObject().(type) {
	case *buildapi.Build:
		buildapi.SetBuildForcePull(obj.Spec.Strategy)
	case *buildapi.BuildRequest:
		setBuildRequestForcePull(obj)
	default:
		return errors.NewBadRequest(fmt.Sprintf("Unable to convert %q resource", attr.GetResource()))
	}

	return nil
}

func setBuildRequestForcePull(br *buildapi.BuildRequest) {
	if br.Annotations == nil {
		br.Annotations = make(map[string]string)
	}

	br.Annotations[buildapi.BuildAlwaysPullImagesAnnotation] = "true"
}
