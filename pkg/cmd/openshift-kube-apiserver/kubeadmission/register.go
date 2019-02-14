package kubeadmission

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	mutatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/mutating"
	"k8s.io/kubernetes/plugin/pkg/admission/noderestriction"

	"github.com/openshift/origin/pkg/admission/customresourcevalidation/customresourcevalidationregistration"
	authorizationrestrictusers "github.com/openshift/origin/pkg/authorization/apiserver/admission/restrictusers"
	imagepolicyapi "github.com/openshift/origin/pkg/image/apiserver/admission/apis/imagepolicy"
	"github.com/openshift/origin/pkg/image/apiserver/admission/imagepolicy"
	imageadmission "github.com/openshift/origin/pkg/image/apiserver/admission/limitrange"
	projectnodeenv "github.com/openshift/origin/pkg/project/apiserver/admission/nodeenv"
	overrideapi "github.com/openshift/origin/pkg/quota/apiserver/admission/apis/clusterresourceoverride"
	quotaclusterresourceoverride "github.com/openshift/origin/pkg/quota/apiserver/admission/clusterresourceoverride"
	quotaclusterresourcequota "github.com/openshift/origin/pkg/quota/apiserver/admission/clusterresourcequota"
	quotarunonceduration "github.com/openshift/origin/pkg/quota/apiserver/admission/runonceduration"
	ingressadmission "github.com/openshift/origin/pkg/route/apiserver/admission"
	schedulerpodnodeconstraints "github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints"
	"github.com/openshift/origin/pkg/security/apiserver/admission/sccadmission"
	securityadmission "github.com/openshift/origin/pkg/security/apiserver/admission/sccadmission"
	"github.com/openshift/origin/pkg/service/admission/externalipranger"
	"github.com/openshift/origin/pkg/service/admission/restrictedendpoints"
)

func RegisterOpenshiftKubeAdmissionPlugins(plugins *admission.Plugins) {
	authorizationrestrictusers.Register(plugins)
	imagepolicy.Register(plugins)
	ingressadmission.Register(plugins)
	projectnodeenv.Register(plugins)
	quotaclusterresourceoverride.Register(plugins)
	quotaclusterresourcequota.Register(plugins)
	quotarunonceduration.Register(plugins)
	schedulerpodnodeconstraints.Register(plugins)
	securityadmission.Register(plugins)
	securityadmission.RegisterSCCExecRestrictions(plugins)
	externalipranger.RegisterExternalIP(plugins)
	externalipranger.DeprecatedRegisterExternalIP(plugins)
	restrictedendpoints.RegisterRestrictedEndpoints(plugins)
	restrictedendpoints.DeprecatedRegisterRestrictedEndpoints(plugins)
}

var (

	// these are admission plugins that cannot be applied until after the kubeapiserver starts.
	// TODO if nothing comes to mind in 3.10, kill this
	SkipRunLevelZeroPlugins = sets.NewString()
	// these are admission plugins that cannot be applied until after the openshiftapiserver apiserver starts.
	SkipRunLevelOnePlugins = sets.NewString(
		"project.openshift.io/ProjectRequestLimit",
		"authorization.openshift.io/RestrictSubjectBindings",
		"quota.openshift.io/ClusterResourceQuota",
		"image.openshift.io/ImagePolicy",
		overrideapi.PluginName, // "autoscaling.openshift.io/ClusterResourceOverride"
		"scheduling.openshift.io/OriginPodNodeEnvironment",
		"autoscaling.openshift.io/RunOnceDuration",
		sccadmission.PluginName, // "security.openshift.io/SecurityContextConstraint"
		"security.openshift.io/SCCExecRestrictions",
	)

	// AfterKubeAdmissionPlugins are the admission plugins to add after kube admission, before mutating webhooks
	openshiftAdmissionPluginsForKube = []string{
		"autoscaling.openshift.io/ClusterResourceOverride",
		"authorization.openshift.io/RestrictSubjectBindings",
		"autoscaling.openshift.io/RunOnceDuration",
		"scheduling.openshift.io/PodNodeConstraints",
		"scheduling.openshift.io/OriginPodNodeEnvironment",
		externalipranger.ExternalIPPluginName,
		"ExternalIPRanger",
		restrictedendpoints.RestrictedEndpointsPluginName,
		"openshift.io/RestrictedEndpointsAdmission",
		"image.openshift.io/ImagePolicy",
		sccadmission.PluginName,
		"security.openshift.io/SCCExecRestrictions",
		ingressadmission.IngressAdmission,
		"quota.openshift.io/ClusterResourceQuota",
	}

	// additionalDefaultOnPlugins is a list of plugins we turn on by default that core kube does not.
	additionalDefaultOnPlugins = sets.NewString(
		imageadmission.PluginName, // "image.openshift.io/ImageLimitRange"
		"scheduling.openshift.io/OriginPodNodeEnvironment",
		"PodNodeSelector",
		"Priority",
		externalipranger.ExternalIPPluginName,
		"ExternalIPRanger",
		restrictedendpoints.RestrictedEndpointsPluginName,
		"openshift.io/RestrictedEndpointsAdmission",
		noderestriction.PluginName,
		securityadmission.PluginName,
		"StorageObjectInUseProtection",
		"security.openshift.io/SCCExecRestrictions",
		"PersistentVolumeLabel",
		"OwnerReferencesPermissionEnforcement",
		"PodTolerationRestriction",
		"quota.openshift.io/ClusterResourceQuota",
		"route.openshift.io/IngressAdmission",
		"project.openshift.io/ProjectRequestLimit",
		"autoscaling.openshift.io/RunOnceDuration",
		"scheduling.openshift.io/PodNodeConstraints",
		overrideapi.PluginName,
		imagepolicyapi.PluginName,
	)
)

func NewOrderedKubeAdmissionPlugins(kubeAdmissionOrder []string) []string {
	ret := []string{}
	for _, curr := range kubeAdmissionOrder {
		if curr == mutatingwebhook.PluginName {
			ret = append(ret, openshiftAdmissionPluginsForKube...)
			ret = append(ret, customresourcevalidationregistration.AllCustomResourceValidators...)
		}
		ret = append(ret, curr)
	}
	return ret
}

func NewDefaultOffPluginsFunc(kubeDefaultOffAdmission sets.String) func() sets.String {
	return func() sets.String {
		kubeOff := sets.NewString(kubeDefaultOffAdmission.UnsortedList()...)
		kubeOff.Insert("authorization.openshift.io/RestrictSubjectBindings")
		kubeOff.Delete(additionalDefaultOnPlugins.List()...)
		return kubeOff
	}
}
