package customresourcevalidationregistration

import (
	"k8s.io/apiserver/pkg/admission"

	"github.com/openshift/origin/pkg/admission/customresourcevalidation/apiserver"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/authentication"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/clusterresourcequota"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/config"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/console"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/features"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/image"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/oauth"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/project"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/rolebindingrestriction"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/scheduler"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/securitycontextconstraints"
)

// AllCustomResourceValidators are the names of all custom resource validators that should be registered
var AllCustomResourceValidators = []string{
	apiserver.PluginName,
	authentication.PluginName,
	features.PluginName,
	console.PluginName,
	image.PluginName,
	oauth.PluginName,
	project.PluginName,
	config.PluginName,
	scheduler.PluginName,
	clusterresourcequota.PluginName,
	securitycontextconstraints.PluginName,
	rolebindingrestriction.PluginName,

	// this one is special because we don't work without it.
	securitycontextconstraints.DefaultingPluginName,
}

func RegisterCustomResourceValidation(plugins *admission.Plugins) {
	apiserver.Register(plugins)
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
	// This plugin validates the security.openshift.io/v1 SecurityContextConstraints resources.
	securitycontextconstraints.Register(plugins)
	// This plugin validates the authorization.openshift.io/v1 RoleBindingRestriction resources.
	rolebindingrestriction.Register(plugins)

	// this one is special because we don't work without it.
	securitycontextconstraints.RegisterDefaulting(plugins)
}
