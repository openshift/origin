package kubeadmission

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	mutatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/mutating"
	"k8s.io/kubernetes/plugin/pkg/admission/resourcequota"

	"github.com/openshift/origin/pkg/admission/customresourcevalidation/customresourcevalidationregistration"
	authorizationrestrictusers "github.com/openshift/origin/pkg/authorization/apiserver/admission/restrictusers"
	quotaclusterresourceoverride "github.com/openshift/origin/pkg/autoscaling/admission/clusterresourceoverride"
	quotarunonceduration "github.com/openshift/origin/pkg/autoscaling/admission/runonceduration"
	imagepolicyapiv1 "github.com/openshift/origin/pkg/image/apiserver/admission/apis/imagepolicy/v1"
	"github.com/openshift/origin/pkg/image/apiserver/admission/imagepolicy"
	"github.com/openshift/origin/pkg/network/admission/externalipranger"
	"github.com/openshift/origin/pkg/network/admission/restrictedendpoints"
	quotaclusterresourcequota "github.com/openshift/origin/pkg/quota/apiserver/admission/clusterresourcequota"
	ingressadmission "github.com/openshift/origin/pkg/route/apiserver/admission"
	projectnodeenv "github.com/openshift/origin/pkg/scheduler/admission/nodeenv"
	schedulerpodnodeconstraints "github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints"
	securityadmission "github.com/openshift/origin/pkg/security/apiserver/admission/sccadmission"
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
	restrictedendpoints.RegisterRestrictedEndpoints(plugins)
}

var (

	// these are admission plugins that cannot be applied until after the kubeapiserver starts.
	// TODO if nothing comes to mind in 3.10, kill this
	SkipRunLevelZeroPlugins = sets.NewString()
	// these are admission plugins that cannot be applied until after the openshiftapiserver apiserver starts.
	SkipRunLevelOnePlugins = sets.NewString(
		imagepolicyapiv1.PluginName, // "image.openshift.io/ImagePolicy"
		"quota.openshift.io/ClusterResourceQuota",
		"security.openshift.io/SecurityContextConstraint",
		"security.openshift.io/SCCExecRestrictions",
	)

	// openshiftAdmissionPluginsForKubeBeforeMutating are the admission plugins to add after kube admission, before mutating webhooks
	openshiftAdmissionPluginsForKubeBeforeMutating = []string{
		"autoscaling.openshift.io/ClusterResourceOverride",
		"authorization.openshift.io/RestrictSubjectBindings",
		"autoscaling.openshift.io/RunOnceDuration",
		"scheduling.openshift.io/PodNodeConstraints",
		"scheduling.openshift.io/OriginPodNodeEnvironment",
		"network.openshift.io/ExternalIPRanger",
		"network.openshift.io/RestrictedEndpointsAdmission",
		imagepolicyapiv1.PluginName, // "image.openshift.io/ImagePolicy"
		"security.openshift.io/SecurityContextConstraint",
		"security.openshift.io/SCCExecRestrictions",
		"route.openshift.io/IngressAdmission",
	}

	// openshiftAdmissionPluginsForKubeAfterResourceQuota are the plugins to add after ResourceQuota plugin
	openshiftAdmissionPluginsForKubeAfterResourceQuota = []string{
		"quota.openshift.io/ClusterResourceQuota",
	}

	// additionalDefaultOnPlugins is a list of plugins we turn on by default that core kube does not.
	additionalDefaultOnPlugins = sets.NewString(
		"NodeRestriction",
		"OwnerReferencesPermissionEnforcement",
		"PersistentVolumeLabel",
		"PodNodeSelector",
		"PodTolerationRestriction",
		"Priority",
		imagepolicyapiv1.PluginName, // "image.openshift.io/ImagePolicy"
		"StorageObjectInUseProtection",
	)
)

func NewOrderedKubeAdmissionPlugins(kubeAdmissionOrder []string) []string {
	ret := []string{}
	for _, curr := range kubeAdmissionOrder {
		if curr == mutatingwebhook.PluginName {
			ret = append(ret, openshiftAdmissionPluginsForKubeBeforeMutating...)
			ret = append(ret, customresourcevalidationregistration.AllCustomResourceValidators...)
		}

		ret = append(ret, curr)

		if curr == resourcequota.PluginName {
			ret = append(ret, openshiftAdmissionPluginsForKubeAfterResourceQuota...)
		}
	}
	return ret
}

func NewDefaultOffPluginsFunc(kubeDefaultOffAdmission sets.String) func() sets.String {
	return func() sets.String {
		kubeOff := sets.NewString(kubeDefaultOffAdmission.UnsortedList()...)
		kubeOff.Delete(additionalDefaultOnPlugins.List()...)
		kubeOff.Delete(openshiftAdmissionPluginsForKubeBeforeMutating...)
		kubeOff.Delete(openshiftAdmissionPluginsForKubeAfterResourceQuota...)
		kubeOff.Delete(customresourcevalidationregistration.AllCustomResourceValidators...)
		return kubeOff
	}
}
