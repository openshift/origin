package customresourcevalidationregistration

import (
	"k8s.io/apiserver/pkg/admission"

	"github.com/openshift/origin/pkg/admission/customresourcevalidation/authentication"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/clusterresourcequota"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/config"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/console"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/features"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/image"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/oauth"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/project"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/scheduler"
)

// AllCustomResourceValidators are the names of all custom resource validators that should be registered
var AllCustomResourceValidators = []string{
	authentication.PluginName,
	features.PluginName,
	console.PluginName,
	image.PluginName,
	oauth.PluginName,
	project.PluginName,
	config.PluginName,
	scheduler.PluginName,
	clusterresourcequota.PluginName,
}

func RegisterCustomResourceValidation(plugins *admission.Plugins) {
	authentication.Register(plugins)
	features.Register(plugins)
	console.Register(plugins)
	image.Register(plugins)
	oauth.Register(plugins)
	project.Register(plugins)
	config.Register(plugins)
	scheduler.Register(plugins)

	// This plugin validates the quota.openshift.io/v1 ClusterResourceQuota resources.
	// NOTE: This is only allowed because it is required to get a running control plane operator.
	clusterresourcequota.Register(plugins)
}
