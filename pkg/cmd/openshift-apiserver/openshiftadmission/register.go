package openshiftadmission

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	mutatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/mutating"
	validatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/validating"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/kubernetes/plugin/pkg/admission/gc"
	"k8s.io/kubernetes/plugin/pkg/admission/resourcequota"

	buildsecretinjector "github.com/openshift/origin/pkg/build/apiserver/admission/secretinjector"
	buildstrategyrestrictions "github.com/openshift/origin/pkg/build/apiserver/admission/strategyrestrictions"
	imagepolicyapi "github.com/openshift/origin/pkg/image/apiserver/admission/apis/imagepolicy"
	"github.com/openshift/origin/pkg/image/apiserver/admission/imagepolicy"
	imageadmission "github.com/openshift/origin/pkg/image/apiserver/admission/limitrange"
	projectrequestlimit "github.com/openshift/origin/pkg/project/apiserver/admission/requestlimit"
	quotaclusterresourcequota "github.com/openshift/origin/pkg/quota/apiserver/admission/clusterresourcequota"
	schedulerpodnodeconstraints "github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints"
)

// TODO register this per apiserver or at least per process
var OriginAdmissionPlugins = admission.NewPlugins()

func init() {
	RegisterAllAdmissionPlugins(OriginAdmissionPlugins)
}

// RegisterAllAdmissionPlugins registers all admission plugins
func RegisterAllAdmissionPlugins(plugins *admission.Plugins) {
	// register kubernetes plugins that are not yet generic
	gc.Register(plugins)            // "OwnerReferencesPermissionEnforcement"
	resourcequota.Register(plugins) // "ResourceQuota"

	// register the generic plugins
	genericapiserver.RegisterAllAdmissionPlugins(plugins)

	// register openshift specific plugins
	RegisterOpenshiftAdmissionPlugins(plugins)
}

func RegisterOpenshiftAdmissionPlugins(plugins *admission.Plugins) {
	projectrequestlimit.Register(plugins)         // "project.openshift.io/ProjectRequestLimit"
	buildsecretinjector.Register(plugins)         // "build.openshift.io/BuildConfigSecretInjector"
	buildstrategyrestrictions.Register(plugins)   // "build.openshift.io/BuildByStrategy"
	imageadmission.Register(plugins)              // "image.openshift.io/ImageLimitRange"
	schedulerpodnodeconstraints.Register(plugins) // "scheduling.openshift.io/PodNodeConstraints"
	imagepolicy.Register(plugins)                 // "image.openshift.io/ImagePolicy"
	quotaclusterresourcequota.Register(plugins)   // "quota.openshift.io/ClusterResourceQuota"
}

var (
	// OpenShiftAdmissionPlugins gives the in-order default admission chain for openshift resources.
	OpenShiftAdmissionPlugins = []string{
		lifecycle.PluginName, // "NamespaceLifecycle"
		"OwnerReferencesPermissionEnforcement",
		"project.openshift.io/ProjectRequestLimit",
		"build.openshift.io/BuildConfigSecretInjector",
		"build.openshift.io/BuildByStrategy",
		imageadmission.PluginName, // "image.openshift.io/ImageLimitRange"
		"scheduling.openshift.io/PodNodeConstraints",
		imagepolicyapi.PluginName,    // "image.openshift.io/ImagePolicy"
		mutatingwebhook.PluginName,   // "MutatingAdmissionWebhook"
		validatingwebhook.PluginName, // "ValidatingAdmissionWebhook"
		"ResourceQuota",
		"quota.openshift.io/ClusterResourceQuota",
	}

	// DefaultOffPlugins includes plugins which require explicit configuration to run
	// if you wire them incorrectly, they may prevent the server from starting
	DefaultOffPlugins = sets.NewString(
		"project.openshift.io/ProjectRequestLimit",
		"scheduling.openshift.io/PodNodeConstraints",
	)
)
