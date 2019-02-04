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
	restrictedendpoints.RegisterRestrictedEndpoints(plugins)
}

var (

	// these are admission plugins that cannot be applied until after the kubeapiserver starts.
	// TODO if nothing comes to mind in 3.10, kill this
	SkipRunLevelZeroPlugins = sets.NewString()
	// these are admission plugins that cannot be applied until after the openshiftapiserver apiserver starts.
	SkipRunLevelOnePlugins = sets.NewString(
		"ProjectRequestLimit",
		"openshift.io/RestrictSubjectBindings",
		"openshift.io/ClusterResourceQuota",
		"openshift.io/ImagePolicy",
		overrideapi.PluginName,
		"OriginPodNodeEnvironment",
		"RunOnceDuration",
		sccadmission.PluginName,
		"SCCExecRestrictions",
	)

	// BeforeKubeAdmissionPlugins is the list of plugins to add before kube admission, so we run before limit ranges
	// TODO this cannot be done on today's mutation
	beforeKubeAdmissionPlugins = []string{
		"ClusterResourceOverride",
	}

	// AfterKubeAdmissionPlugins are the admission plugins to add after kube admission, before mutating webhooks
	afterKubeAdmissionPlugins = []string{
		"openshift.io/RestrictSubjectBindings",
		"RunOnceDuration",
		"PodNodeConstraints",
		"OriginPodNodeEnvironment",
		externalipranger.ExternalIPPluginName,
		restrictedendpoints.RestrictedEndpointsPluginName,
		"openshift.io/ImagePolicy",
		sccadmission.PluginName,
		"SCCExecRestrictions",
		ingressadmission.IngressAdmission,
	}

	// FinalKubeAdmissionPlugins are the admission plugins to add after quota.  You shouldn't ever add this.
	finalKubeAdmissionPlugins = []string{
		"openshift.io/ClusterResourceQuota",
	}

	// additionalDefaultOnPlugins is a list of plugins we turn on by default that core kube does not.
	additionalDefaultOnPlugins = sets.NewString(
		imageadmission.PluginName, // "openshift.io/ImageLimitRange"
		"OriginPodNodeEnvironment",
		"PodNodeSelector",
		"Priority",
		externalipranger.ExternalIPPluginName,
		restrictedendpoints.RestrictedEndpointsPluginName,
		noderestriction.PluginName,
		securityadmission.PluginName,
		"StorageObjectInUseProtection",
		"SCCExecRestrictions",
		"PersistentVolumeLabel",
		"OwnerReferencesPermissionEnforcement",
		"PodTolerationRestriction",
		"openshift.io/ClusterResourceQuota",
		"openshift.io/IngressAdmission",
	)

	// additionalDefaultOffPlugins are admission plugins we choose not to enable by default in openshift
	// you shouldn't put anything from kube in this list without api-approvers signing off on it.
	additionalDefaultOffPlugins = sets.NewString(
		"ProjectRequestLimit",
		"RunOnceDuration",
		"PodNodeConstraints",
		overrideapi.PluginName,
		imagepolicyapi.PluginName,
		"openshift.io/RestrictSubjectBindings",
	)
)

func NewOrderedKubeAdmissionPlugins(kubeAdmissionOrder []string) []string {
	ret := append([]string{}, beforeKubeAdmissionPlugins...)
	for _, curr := range kubeAdmissionOrder {
		if curr == mutatingwebhook.PluginName {
			ret = append(ret, afterKubeAdmissionPlugins...)
			ret = append(ret, customresourcevalidationregistration.AllCustomResourceValidators...)
		}
		ret = append(ret, curr)
	}
	ret = append(ret, finalKubeAdmissionPlugins...)
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
