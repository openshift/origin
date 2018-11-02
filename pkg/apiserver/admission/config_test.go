package admission

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	mutatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/mutating"
	validatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/validating"
	"k8s.io/kubernetes/plugin/pkg/admission/noderestriction"
	expandpvcadmission "k8s.io/kubernetes/plugin/pkg/admission/storage/persistentvolume/resize"
	storageclassdefaultadmission "k8s.io/kubernetes/plugin/pkg/admission/storage/storageclass/setdefault"

	configv1 "github.com/openshift/api/config/v1"
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/image/apiserver/admission/apis/imagepolicy"
	imageadmission "github.com/openshift/origin/pkg/image/apiserver/admission/limitrange"
	overrideapi "github.com/openshift/origin/pkg/quota/apiserver/admission/apis/clusterresourceoverride"
	ingressadmission "github.com/openshift/origin/pkg/route/apiserver/admission"
	"github.com/openshift/origin/pkg/security/apiserver/admission/sccadmission"
	"github.com/openshift/origin/pkg/service/admission/externalipranger"
	"github.com/openshift/origin/pkg/service/admission/restrictedendpoints"
)

// combinedAdmissionControlPlugins gives the in-order default admission chain for all resources resources.
// When possible, this list is used.  The set of openshift+kube chains must exactly match this set.  In addition,
// the order specified in the openshift and kube chains must match the order here.
var combinedAdmissionControlPlugins = []string{
	"AlwaysAdmit",
	"NamespaceAutoProvision",
	"NamespaceExists",
	lifecycle.PluginName,
	"EventRateLimit",
	"ProjectRequestLimit",
	"openshift.io/RestrictSubjectBindings",
	"openshift.io/BuildConfigSecretInjector",
	"BuildByStrategy",
	imageadmission.PluginName,
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

// TestAdmissionPluginChains makes sure that the admission plugin lists are coherent.
// we have to maintain three different lists of plugins: default origin, default kube, default combined
// the set of (default origin and default kube) and default combined, but must be equal
// the order of default origin must follow the order of default combined
// the order of default kube must follow the order of default combined
func TestAdmissionPluginChains(t *testing.T) {
	individualSet := sets.NewString(OpenShiftAdmissionPlugins...)
	individualSet.Insert(KubeAdmissionPlugins...)
	combinedSet := sets.NewString(combinedAdmissionControlPlugins...)

	if !individualSet.Equal(combinedSet) {
		t.Fatalf("individualSets are missing: %v combinedSet is missing: %v", combinedSet.Difference(individualSet), individualSet.Difference(combinedSet))
	}

	lastCurrIndex := -1
	for _, plugin := range OpenShiftAdmissionPlugins {
		for lastCurrIndex = lastCurrIndex + 1; lastCurrIndex < len(combinedAdmissionControlPlugins); lastCurrIndex++ {
			if combinedAdmissionControlPlugins[lastCurrIndex] == plugin {
				break
			}
		}

		if lastCurrIndex >= len(combinedAdmissionControlPlugins) {
			t.Errorf("openshift admission plugins are out of order compared to the combined list.  Failed at %v", plugin)
		}
	}

	lastCurrIndex = -1
	for _, plugin := range KubeAdmissionPlugins {
		for lastCurrIndex = lastCurrIndex + 1; lastCurrIndex < len(combinedAdmissionControlPlugins); lastCurrIndex++ {
			if combinedAdmissionControlPlugins[lastCurrIndex] == plugin {
				break
			}
		}

		if lastCurrIndex >= len(combinedAdmissionControlPlugins) {
			t.Errorf("kube admission plugins are out of order compared to the combined list.  Failed at %v", plugin)
		}
	}
}

// legacyOpenshiftAdmissionPlugins holds names that already existed without a prefix.  We should come up with a migration
// plan (double register for a few releases?), but for now just make sure we don't get worse.
var legacyOpenshiftAdmissionPlugins = sets.NewString(
	"ProjectRequestLimit",
	"PodNodeConstraints",
	"BuildByStrategy",
	"RunOnceDuration",
	"OriginPodNodeEnvironment",
	overrideapi.PluginName,
	externalipranger.ExternalIPPluginName,
	sccadmission.PluginName,
	"SCCExecRestrictions",
	"ResourceQuota",
)

// TestAdmissionPluginNames makes sure that openshift admission plugins are prefixed with `openshift.io/`.
func TestAdmissionPluginNames(t *testing.T) {
	originAdmissionPlugins := admission.NewPlugins()
	RegisterOpenshiftAdmissionPlugins(originAdmissionPlugins)

	for _, plugin := range originAdmissionPlugins.Registered() {
		if !strings.HasPrefix(plugin, "openshift.io/") && !legacyOpenshiftAdmissionPlugins.Has(plugin) {
			t.Errorf("openshift admission plugins must be prefixed with openshift.io/ %v", plugin)
		}
	}
}

func TestUnusuedKubeAdmissionPlugins(t *testing.T) {
	allAdmissionPlugins := sets.NewString(OriginAdmissionPlugins.Registered()...)
	knownAdmissionPlugins := sets.NewString(combinedAdmissionControlPlugins...)

	if unorderedPlugins := allAdmissionPlugins.Difference(knownAdmissionPlugins); len(unorderedPlugins) != 0 {
		t.Errorf("%v need to be ordered and enabled/disabled", unorderedPlugins.List())
	}
}

func TestSeparateAdmissionChainDetection(t *testing.T) {
	testCases := []struct {
		name                  string
		options               openshiftcontrolplanev1.OpenShiftAPIServerConfig
		admissionChainBuilder func(pluginNames []string, admissionConfigFilename string, pluginInitializer admission.PluginInitializer, decorator admission.Decorator) (admission.Interface, error)
	}{
		{
			name:    "stock everything",
			options: openshiftcontrolplanev1.OpenShiftAPIServerConfig{},
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, pluginInitializer admission.PluginInitializer, decorator admission.Decorator) (admission.Interface, error) {
				if !reflect.DeepEqual(pluginNames, combinedAdmissionControlPlugins) {
					t.Errorf("%s: expected %v, got %v", "stock everything", combinedAdmissionControlPlugins, pluginNames)
				}
				return nil, nil
			},
		},
		{
			name: "specified kube admission config file",
			options: openshiftcontrolplanev1.OpenShiftAPIServerConfig{
				APIServerArguments: configapi.ExtendedArguments{
					"admission-control-config-file": []string{"foo"},
				},
			},
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, pluginInitializer admission.PluginInitializer, decorator admission.Decorator) (admission.Interface, error) {
				kubePlugins := difference(KubeAdmissionPlugins, DefaultOffPlugins)
				originPlugins := difference(OpenShiftAdmissionPlugins, DefaultOffPlugins)
				isKube := reflect.DeepEqual(pluginNames, kubePlugins)
				isOrigin := reflect.DeepEqual(pluginNames, originPlugins)
				if !isKube && !isOrigin {
					t.Errorf("%s: expected either %v or %v, got %v", "specified conflicting plugin configs 01", kubePlugins, originPlugins, pluginNames)
				}
				return nil, nil
			},
		},
		{
			name: "specified, non-conflicting plugin configs 01",
			options: openshiftcontrolplanev1.OpenShiftAPIServerConfig{
				GenericAPIServerConfig: configv1.GenericAPIServerConfig{
					AdmissionPluginConfig: map[string]configv1.AdmissionPluginConfig{
						"foo": {
							Location: "bar",
						},
					},
				},
			},
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, pluginInitializer admission.PluginInitializer, decorator admission.Decorator) (admission.Interface, error) {
				if !reflect.DeepEqual(pluginNames, combinedAdmissionControlPlugins) {
					t.Errorf("%s: expected %v, got %v", "specified, non-conflicting plugin configs 01", combinedAdmissionControlPlugins, pluginNames)
				}
				return nil, nil
			},
		},
	}

	for _, tc := range testCases {
		newAdmissionChainFunc = tc.admissionChainBuilder
		_, _ = NewAdmissionChains(tc.options.APIServerArguments["admission-control-config-file"], []string{}, []string{}, tc.options.AdmissionPluginConfig, nil, nil)
	}
}

func difference(listA []string, setB sets.String) []string {
	result := []string{}
	for _, a := range listA {
		if !setB.Has(a) {
			result = append(result, a)
		}
	}
	return result
}
func TestQuotaAdmissionPluginsAreLast(t *testing.T) {
	kubeLen := len(KubeAdmissionPlugins)
	if kubeLen < 2 {
		t.Fatalf("must have at least the 2 quota plugins")
	}

	if KubeAdmissionPlugins[kubeLen-2] != "ResourceQuota" {
		t.Errorf("kubeAdmissionPlugins must have %s as the next to last plugin", "ResourceQuota")
	}

	if KubeAdmissionPlugins[kubeLen-1] != "openshift.io/ClusterResourceQuota" {
		t.Errorf("kubeAdmissionPlugins must have ClusterResourceQuota as the last plugin")
	}

	combinedLen := len(combinedAdmissionControlPlugins)
	if combinedAdmissionControlPlugins[combinedLen-2] != "ResourceQuota" {
		t.Errorf("combinedAdmissionControlPlugins must have %s as the next to last plugin", "ResourceQuota")
	}

	if combinedAdmissionControlPlugins[combinedLen-1] != "openshift.io/ClusterResourceQuota" {
		t.Errorf("combinedAdmissionControlPlugins must have ClusterResourceQuota as the last plugin")
	}
}
func TestFixupAdmissionPlugins(t *testing.T) {
	inputList := []string{"DefaultTolerationSeconds", "openshift.io/OriginResourceQuota", "OwnerReferencesPermissionEnforcement", "ResourceQuota", "openshift.io/ClusterResourceQuota"}
	expectedList := []string{"DefaultTolerationSeconds", "OwnerReferencesPermissionEnforcement", "ResourceQuota", "openshift.io/ClusterResourceQuota"}
	actualList := fixupAdmissionPlugins(inputList)
	if !reflect.DeepEqual(expectedList, actualList) {
		t.Errorf("Expected: %v, but got: %v", expectedList, actualList)
	}
}
