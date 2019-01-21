package customresourcevalidationregistration

import (
	"k8s.io/apiserver/pkg/admission"

	"github.com/openshift/origin/pkg/admission/customresourcevalidation/authentication"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/image"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/project"
)

// AllCustomResourceValidators are the names of all custom resource validators that should be registered
var AllCustomResourceValidators = []string{
	image.PluginName,
	project.PluginName,
	authentication.PluginName,
}

func RegisterCustomResourceValidation(plugins *admission.Plugins) {
	image.Register(plugins)
	project.Register(plugins)
	authentication.Register(plugins)
}
