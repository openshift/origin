package admission

import (
	"io"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	genericapiserver "k8s.io/apiserver/pkg/server"
	kubeapiserver "k8s.io/kubernetes/pkg/kubeapiserver/options"

	// Admission control plug-ins used by OpenShift
	authorizationrestrictusers "github.com/openshift/origin/pkg/authorization/apiserver/admission/restrictusers"
	buildjenkinsbootstrapper "github.com/openshift/origin/pkg/build/apiserver/admission/jenkinsbootstrapper"
	buildsecretinjector "github.com/openshift/origin/pkg/build/apiserver/admission/secretinjector"
	buildstrategyrestrictions "github.com/openshift/origin/pkg/build/apiserver/admission/strategyrestrictions"
	imagepolicy "github.com/openshift/origin/pkg/image/apiserver/admission/imagepolicy"
	imageadmission "github.com/openshift/origin/pkg/image/apiserver/admission/limitrange"
	ingressadmission "github.com/openshift/origin/pkg/network/apiserver/admission"
	projectnodeenv "github.com/openshift/origin/pkg/project/apiserver/admission/nodeenv"
	projectrequestlimit "github.com/openshift/origin/pkg/project/apiserver/admission/requestlimit"
	quotaclusterresourceoverride "github.com/openshift/origin/pkg/quota/apiserver/admission/clusterresourceoverride"
	quotaclusterresourcequota "github.com/openshift/origin/pkg/quota/apiserver/admission/clusterresourcequota"
	quotarunonceduration "github.com/openshift/origin/pkg/quota/apiserver/admission/runonceduration"
	schedulerpodnodeconstraints "github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints"
	securityadmission "github.com/openshift/origin/pkg/security/apiserver/admission/sccadmission"

	"k8s.io/kubernetes/plugin/pkg/admission/noderestriction"
	expandpvcadmission "k8s.io/kubernetes/plugin/pkg/admission/storage/persistentvolume/resize"
	storageclassdefaultadmission "k8s.io/kubernetes/plugin/pkg/admission/storage/storageclass/setdefault"

	imagepolicyapi "github.com/openshift/origin/pkg/image/apiserver/admission/apis/imagepolicy"
	overrideapi "github.com/openshift/origin/pkg/quota/apiserver/admission/apis/clusterresourceoverride"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"

	configlatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/service/admission/externalipranger"
	"github.com/openshift/origin/pkg/service/admission/restrictedendpoints"
)

// TODO register this per apiserver or at least per process
var OriginAdmissionPlugins = admission.NewPlugins()

func init() {
	RegisterAllAdmissionPlugins(OriginAdmissionPlugins)
}

// RegisterAllAdmissionPlugins registers all admission plugins
func RegisterAllAdmissionPlugins(plugins *admission.Plugins) {
	kubeapiserver.RegisterAllAdmissionPlugins(plugins)
	genericapiserver.RegisterAllAdmissionPlugins(plugins)
	RegisterOpenshiftAdmissionPlugins(plugins)
}

func RegisterOpenshiftAdmissionPlugins(plugins *admission.Plugins) {
	authorizationrestrictusers.Register(plugins)
	buildjenkinsbootstrapper.Register(plugins)
	buildsecretinjector.Register(plugins)
	buildstrategyrestrictions.Register(plugins)
	imageadmission.Register(plugins)
	imagepolicy.Register(plugins)
	ingressadmission.Register(plugins)
	projectnodeenv.Register(plugins)
	projectrequestlimit.Register(plugins)
	quotaclusterresourceoverride.Register(plugins)
	quotaclusterresourcequota.Register(plugins)
	quotarunonceduration.Register(plugins)
	schedulerpodnodeconstraints.Register(plugins)
	securityadmission.Register(plugins)
	securityadmission.RegisterSCCExecRestrictions(plugins)
	externalipranger.RegisterExternalIP(plugins)
	restrictedendpoints.RegisterRestrictedEndpoints(plugins)
}

var (
	DefaultOnPlugins = sets.NewString(
		"openshift.io/JenkinsBootstrapper",
		"openshift.io/BuildConfigSecretInjector",
		"BuildByStrategy",
		storageclassdefaultadmission.PluginName,
		imageadmission.PluginName,
		lifecycle.PluginName,
		"OriginPodNodeEnvironment",
		"PodNodeSelector",
		"Priority",
		externalipranger.ExternalIPPluginName,
		restrictedendpoints.RestrictedEndpointsPluginName,
		"LimitRanger",
		"ServiceAccount",
		noderestriction.PluginName,
		securityadmission.PluginName,
		"StorageObjectInUseProtection",
		"SCCExecRestrictions",
		"PersistentVolumeLabel",
		"DefaultStorageClass",
		"OwnerReferencesPermissionEnforcement",
		"PodTolerationRestriction",
		"ResourceQuota",
		"openshift.io/ClusterResourceQuota",
		"openshift.io/IngressAdmission",
		expandpvcadmission.PluginName,
	)

	// DefaultOffPlugins includes plugins which require explicit configuration to run
	// if you wire them incorrectly, they may prevent the server from starting
	DefaultOffPlugins = sets.NewString(
		"ProjectRequestLimit",
		"RunOnceDuration",
		"PodNodeConstraints",
		overrideapi.PluginName,
		imagepolicyapi.PluginName,
		"AlwaysPullImages",
		"ImagePolicyWebhook",
		"openshift.io/RestrictSubjectBindings",
		"LimitPodHardAntiAffinityTopology",
		"DefaultTolerationSeconds",
		"PodPreset", // default to off while PodPreset is alpha
		"EventRateLimit",
		"PodSecurityPolicy",
		"Initializers",
		"ValidatingAdmissionWebhook",
		"MutatingAdmissionWebhook",
		"ExtendedResourceToleration",

		// these should usually be off.
		"AlwaysAdmit",
		"AlwaysDeny",
		"DenyEscalatingExec",
		"DenyExecOnPrivileged",
		"NamespaceAutoProvision",
		"NamespaceExists",
		"SecurityContextDeny",
	)
)

func init() {
	admission.PluginEnabledFn = IsAdmissionPluginActivated
}

func IsAdmissionPluginActivated(name string, config io.Reader) bool {
	// only intercept if we have an explicit enable or disable.  If the check fails in any way,
	// assume that the config was a different type and let the actual admission plugin check it
	if DefaultOnPlugins.Has(name) {
		if enabled, err := configlatest.IsAdmissionPluginActivated(config, true); err == nil && !enabled {
			glog.V(2).Infof("Admission plugin %v is disabled.  It will not be started.", name)
			return false
		}
	} else if DefaultOffPlugins.Has(name) {
		if enabled, err := configlatest.IsAdmissionPluginActivated(config, false); err == nil && !enabled {
			glog.V(2).Infof("Admission plugin %v is not enabled.  It will not be started.", name)
			return false
		}
	}

	return true
}
