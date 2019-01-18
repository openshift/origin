package customresourcevalidationregistration

import (
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/image"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/proxy"
	"k8s.io/apiserver/pkg/admission"
)

// AllCustomResourceValidators are the names of all custom resource validators that should be registered
var AllCustomResourceValidators = []string{
	"config.openshift.io/ValidateImage",
	"config.openshift.io/ValidateProxy",
}

func RegisterCustomResourceValidation(plugins *admission.Plugins) {
	image.Register(plugins)
	proxy.Register(plugins)
}
