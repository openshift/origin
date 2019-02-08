package kubeadmission

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	mutatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/mutating"
	"k8s.io/kubernetes/plugin/pkg/admission/noderestriction"

	"github.com/openshift/origin/pkg/admission/customresourcevalidation/customresourcevalidationregistration"
	authorizationrestrictusers "github.com/openshift/origin/pkg/authorization/apiserver/admission/restrictusers"
	"github.com/openshift/origin/pkg/image/apiserver/admission/imagepolicy"
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
	authorizationrestrictusers.Register(plugins)   // "authorization.openshift.io/RestrictSubjectBindings"
	quotaclusterresourceoverride.Register(plugins) // "autoscaling.openshift.io/ClusterResourceOverride"
	quotarunonceduration.Register(plugins)         // "autoscaling.openshift.io/RunOnceDuration"
	imagepolicy.Register(plugins)                  // "image.openshift.io/ImagePolicy"
	externalipranger.RegisterExternalIP(plugins)   // "network.openshift.io/ExternalIPRanger"
	externalipranger.DeprecatedRegisterExternalIP(plugins)
	restrictedendpoints.RegisterRestrictedEndpoints(plugins) // "network.openshift.io/RestrictedEndpointsAdmission"
	restrictedendpoints.DeprecatedRegisterRestrictedEndpoints(plugins)
	quotaclusterresourcequota.Register(plugins)            // "quota.openshift.io/ClusterResourceQuota"
	ingressadmission.Register(plugins)                     // "route.openshift.io/IngressAdmission"
	projectnodeenv.Register(plugins)                       // "scheduling.openshift.io/OriginPodNodeEnvironment"
	schedulerpodnodeconstraints.Register(plugins)          // "scheduling.openshift.io/PodNodeConstraints"
	securityadmission.Register(plugins)                    // "security.openshift.io/SecurityContextConstraint"
	securityadmission.RegisterSCCExecRestrictions(plugins) // "security.openshift.io/SCCExecRestrictions"
}

var (

	// these are admission plugins that cannot be applied until after the kubeapiserver starts.
	// TODO if nothing comes to mind in 3.10, kill this
	SkipRunLevelZeroPlugins = sets.NewString()
	// these are admission plugins that cannot be applied until after the openshiftapiserver apiserver starts.
	SkipRunLevelOnePlugins = sets.NewString(
		"authorization.openshift.io/RestrictSubjectBindings",
		overrideapi.PluginName, // "autoscaling.openshift.io/ClusterResourceOverride"
		"autoscaling.openshift.io/RunOnceDuration",
		"image.openshift.io/ImagePolicy",
		"quota.openshift.io/ClusterResourceQuota",
		"scheduling.openshift.io/OriginPodNodeEnvironment",
		sccadmission.PluginName, // "security.openshift.io/SecurityContextConstraint"
		"security.openshift.io/SCCExecRestrictions",
	)

	// openshiftAdmissionPluginsForKube are the admission plugins to add after kube admission, before mutating webhooks.
	// this list is ordered first by required order for correctness (partial ordering). DO NOT ALPHABETIZE.
	openshiftAdmissionPluginsForKube = []string{
		"authorization.openshift.io/RestrictSubjectBindings",
		"autoscaling.openshift.io/ClusterResourceOverride",
		"autoscaling.openshift.io/RunOnceDuration",
		externalipranger.ExternalIPPluginName,             // "network.openshift.io/ExternalIPRanger"
		"ExternalIPRanger",                                // TODO deprecated to be removed
		restrictedendpoints.RestrictedEndpointsPluginName, // "network.openshift.io/RestrictedEndpointsAdmission"
		"openshift.io/RestrictedEndpointsAdmission",       // TODO deprecated to be removed
		"image.openshift.io/ImagePolicy",
		ingressadmission.IngressAdmission, // "route.openshift.io/IngressAdmission"
		"scheduling.openshift.io/PodNodeConstraints",
		"scheduling.openshift.io/OriginPodNodeEnvironment",
		sccadmission.PluginName, // "security.openshift.io/SecurityContextConstraint"
		"security.openshift.io/SCCExecRestrictions",

		// quota always comes last because of relative validation cost.
		"quota.openshift.io/ClusterResourceQuota",
	}

	// additionalDefaultOnPlugins is a list of plugins that core kube explicitly disables
	additionalDefaultOnPlugins = sets.NewString(
		noderestriction.PluginName,             // "NodeRestriction"
		"OwnerReferencesPermissionEnforcement", // master
		"PersistentVolumeLabel",                // storage
		"PodNodeSelector",                      // scheduling
		"PodTolerationRestriction",             // scheduling
		"Priority",                             // scheduling
		"StorageObjectInUseProtection",         //storage
	)

	// additionalDefaultOffPlugins are admission plugins we choose not to enable by default in openshift
	// you shouldn't put anything from kube in this list without api-approvers signing off on it.
	additionalDefaultOffPlugins = sets.NewString(
		"authorization.openshift.io/RestrictSubjectBindings",
		overrideapi.PluginName, // "autoscaling.openshift.io/ClusterResourceOverride"
		"autoscaling.openshift.io/RunOnceDuration",
		"scheduling.openshift.io/PodNodeConstraints",
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
		kubeOff.Insert(additionalDefaultOffPlugins.List()...)
		kubeOff.Delete(additionalDefaultOnPlugins.List()...)
		return kubeOff
	}
}
