package customresourcevalidationregistration

import (
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/image"
	"k8s.io/apiserver/pkg/admission"
)

var AllCustomResourceValidators = []string{
	"config.openshift.io/ValidateImage",
}

func RegisterCustomResourceValidation(plugins *admission.Plugins) {
	image.Register(plugins)
}
