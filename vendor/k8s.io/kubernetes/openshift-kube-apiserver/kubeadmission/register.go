package kubeadmission

import (
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/security/sccadmission"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	mutatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/mutating"

	authorizationrestrictusers "k8s.io/kubernetes/openshift-kube-apiserver/admission/authorization/restrictusers"
	quotaclusterresourceoverride "k8s.io/kubernetes/openshift-kube-apiserver/admission/autoscaling/clusterresourceoverride"
	quotarunonceduration "k8s.io/kubernetes/openshift-kube-apiserver/admission/autoscaling/runonceduration"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/customresourcevalidation/customresourcevalidationregistration"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/imagepolicy"
	imagepolicyapiv1 "k8s.io/kubernetes/openshift-kube-apiserver/admission/imagepolicy/apis/imagepolicy/v1"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/network/externalipranger"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/network/restrictedendpoints"
	quotaclusterresourcequota "k8s.io/kubernetes/openshift-kube-apiserver/admission/quota/clusterresourcequota"
	ingressadmission "k8s.io/kubernetes/openshift-kube-apiserver/admission/route"
	projectnodeenv "k8s.io/kubernetes/openshift-kube-apiserver/admission/scheduler/nodeenv"
	schedulerpodnodeconstraints "k8s.io/kubernetes/openshift-kube-apiserver/admission/scheduler/podnodeconstraints"
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
	sccadmission.Register(plugins)
	sccadmission.RegisterSCCExecRestrictions(plugins)
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

	// AfterKubeAdmissionPlugins are the admission plugins to add after kube admission, before mutating webhooks
	openshiftAdmissionPluginsForKube = []string{
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
		kubeOff.Delete(additionalDefaultOnPlugins.List()...)
		kubeOff.Delete(openshiftAdmissionPluginsForKube...)
		kubeOff.Delete(customresourcevalidationregistration.AllCustomResourceValidators...)
		return kubeOff
	}
}
