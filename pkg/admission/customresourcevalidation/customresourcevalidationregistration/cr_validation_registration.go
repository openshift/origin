package customresourcevalidationregistration

import (
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/image"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/project"
	"k8s.io/apiserver/pkg/admission"
)

// AllCustomResourceValidators are the names of all custom resource validators that should be registered
var AllCustomResourceValidators = []string{
	"config.openshift.io/ValidateImage",
	"config.openshift.io/ValidateProject",
}

func RegisterCustomResourceValidation(plugins *admission.Plugins) {
	image.Register(plugins)
	project.Register(plugins)
}
