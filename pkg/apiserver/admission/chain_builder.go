package admission

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	mutatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/mutating"
	validatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/validating"
	"k8s.io/apiserver/pkg/apis/apiserver"
	"k8s.io/kubernetes/plugin/pkg/admission/noderestriction"
	expandpvcadmission "k8s.io/kubernetes/plugin/pkg/admission/storage/persistentvolume/resize"
	storageclassdefaultadmission "k8s.io/kubernetes/plugin/pkg/admission/storage/storageclass/setdefault"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/image/apiserver/admission/apis/imagepolicy"
	imageadmission "github.com/openshift/origin/pkg/image/apiserver/admission/limitrange"
	overrideapi "github.com/openshift/origin/pkg/quota/apiserver/admission/apis/clusterresourceoverride"
	ingressadmission "github.com/openshift/origin/pkg/route/apiserver/admission"
	"github.com/openshift/origin/pkg/security/apiserver/admission/sccadmission"
	"github.com/openshift/origin/pkg/service/admission/externalipranger"
	"github.com/openshift/origin/pkg/service/admission/restrictedendpoints"
)

var (
	// these are admission plugins that cannot be applied until after the kubeapiserver starts.
	// TODO if nothing comes to mind in 3.10, kill this
	SkipRunLevelZeroPlugins = sets.NewString()
	// these are admission plugins that cannot be applied until after the openshiftapiserver apiserver starts.
	SkipRunLevelOnePlugins = sets.NewString(
		"ProjectRequestLimit",
		"openshift.io/RestrictSubjectBindings",
		"openshift.io/ClusterResourceQuota",
		imagepolicy.PluginName,
		overrideapi.PluginName,
		"OriginPodNodeEnvironment",
		"RunOnceDuration",
		sccadmission.PluginName,
		"SCCExecRestrictions",
	)

	// OpenShiftAdmissionPlugins gives the in-order default admission chain for openshift resources.
	OpenShiftAdmissionPlugins = []string{
		lifecycle.PluginName,
		"ProjectRequestLimit",
		"openshift.io/BuildConfigSecretInjector",
		"BuildByStrategy",
		imageadmission.PluginName,
		"PodNodeConstraints",
		"OwnerReferencesPermissionEnforcement",
		"Initializers",
		"MutatingAdmissionWebhook",
		"ValidatingAdmissionWebhook",
		"ResourceQuota",
		"openshift.io/ClusterResourceQuota",
	}

	// KubeAdmissionPlugins gives the in-order default admission chain for kube resources.
	KubeAdmissionPlugins = []string{
		"AlwaysAdmit",
		"NamespaceAutoProvision",
		"NamespaceExists",
		lifecycle.PluginName,
		"EventRateLimit",
		"openshift.io/RestrictSubjectBindings",
		"RunOnceDuration",
		"PodNodeConstraints",
		"OriginPodNodeEnvironment",
		"PodNodeSelector",
		overrideapi.PluginName,
		externalipranger.ExternalIPPluginName,
		restrictedendpoints.RestrictedEndpointsPluginName,
		imagepolicy.PluginName,
		"ImagePolicyWebhook",
		"PodPreset",
		"LimitRanger",
		"ServiceAccount",
		noderestriction.PluginName,
		"SecurityContextDeny",
		sccadmission.PluginName,
		"PodSecurityPolicy",
		"DenyEscalatingExec",
		"DenyExecOnPrivileged",
		storageclassdefaultadmission.PluginName,
		expandpvcadmission.PluginName,
		"AlwaysPullImages",
		"LimitPodHardAntiAffinityTopology",
		"SCCExecRestrictions",
		"PersistentVolumeLabel",
		"OwnerReferencesPermissionEnforcement",
		ingressadmission.IngressAdmission,
		"Priority",
		"ExtendedResourceToleration",
		"DefaultTolerationSeconds",
		"StorageObjectInUseProtection",
		"Initializers",
		mutatingwebhook.PluginName,
		validatingwebhook.PluginName,
		"PodTolerationRestriction",
		"AlwaysDeny",
		// NOTE: ResourceQuota and ClusterResourceQuota must be the last 2 plugins.
		// DO NOT ADD ANY PLUGINS AFTER THIS LINE!
		"ResourceQuota",
		"openshift.io/ClusterResourceQuota",
	}
)

func ConvertOpenshiftAdmissionConfigToKubeAdmissionConfig(in map[string]configv1.AdmissionPluginConfig) (*apiserver.AdmissionConfiguration, error) {
	ret := &apiserver.AdmissionConfiguration{}

	for _, pluginName := range sets.StringKeySet(in).List() {
		openshiftConfig := in[pluginName]

		kubeConfig := apiserver.AdmissionPluginConfiguration{
			Name: pluginName,
			Path: openshiftConfig.Location,
		}

		kubeConfig.Configuration = &runtime.Unknown{
			Raw: openshiftConfig.Configuration.Raw,
		}
		ret.Plugins = append(ret.Plugins, kubeConfig)
	}

	return ret, nil
}
