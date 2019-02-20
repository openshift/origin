package customresourcevalidationregistration

import (
	"k8s.io/apiserver/pkg/admission"

	"github.com/openshift/origin/pkg/admission/customresourcevalidation/authentication"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/features"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/image"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/oauth"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/project"
)

// AllCustomResourceValidators are the names of all custom resource validators that should be registered
var AllCustomResourceValidators = []string{
	authentication.PluginName,
	features.DenyDeleteFeaturesPluginName,
	features.PluginName,
	image.PluginName,
	oauth.PluginName,
	project.PluginName,
}

func RegisterCustomResourceValidation(plugins *admission.Plugins) {
	authentication.Register(plugins)
	features.RegisterDenyDeleteFeatures(plugins)
	features.Register(plugins)
	image.Register(plugins)
	oauth.Register(plugins)
	project.Register(plugins)
}
