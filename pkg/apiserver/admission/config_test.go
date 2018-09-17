package admission

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"

	configv1 "github.com/openshift/api/config/v1"
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	overrideapi "github.com/openshift/origin/pkg/quota/apiserver/admission/apis/clusterresourceoverride"
	"github.com/openshift/origin/pkg/security/apiserver/admission/sccadmission"
	"github.com/openshift/origin/pkg/service/admission/externalipranger"
)

// TestAdmissionPluginChains makes sure that the admission plugin lists are coherent.
// we have to maintain three different lists of plugins: default origin, default kube, default combined
// the set of (default origin and default kube) and default combined, but must be equal
// the order of default origin must follow the order of default combined
// the order of default kube must follow the order of default combined
func TestAdmissionPluginChains(t *testing.T) {
	individualSet := sets.NewString(openshiftAdmissionControlPlugins...)
	individualSet.Insert(KubeAdmissionPlugins...)
	combinedSet := sets.NewString(CombinedAdmissionControlPlugins...)

	if !individualSet.Equal(combinedSet) {
		t.Fatalf("individualSets are missing: %v combinedSet is missing: %v", combinedSet.Difference(individualSet), individualSet.Difference(combinedSet))
	}

	lastCurrIndex := -1
	for _, plugin := range openshiftAdmissionControlPlugins {
		for lastCurrIndex = lastCurrIndex + 1; lastCurrIndex < len(CombinedAdmissionControlPlugins); lastCurrIndex++ {
			if CombinedAdmissionControlPlugins[lastCurrIndex] == plugin {
				break
			}
		}

		if lastCurrIndex >= len(CombinedAdmissionControlPlugins) {
			t.Errorf("openshift admission plugins are out of order compared to the combined list.  Failed at %v", plugin)
		}
	}

	lastCurrIndex = -1
	for _, plugin := range KubeAdmissionPlugins {
		for lastCurrIndex = lastCurrIndex + 1; lastCurrIndex < len(CombinedAdmissionControlPlugins); lastCurrIndex++ {
			if CombinedAdmissionControlPlugins[lastCurrIndex] == plugin {
				break
			}
		}

		if lastCurrIndex >= len(CombinedAdmissionControlPlugins) {
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
	knownAdmissionPlugins := sets.NewString(CombinedAdmissionControlPlugins...)

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
				if !reflect.DeepEqual(pluginNames, CombinedAdmissionControlPlugins) {
					t.Errorf("%s: expected %v, got %v", "stock everything", CombinedAdmissionControlPlugins, pluginNames)
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
				isKube := reflect.DeepEqual(pluginNames, CombinedAdmissionControlPlugins)
				isOrigin := reflect.DeepEqual(pluginNames, CombinedAdmissionControlPlugins)
				if !isKube && !isOrigin {
					t.Errorf("%s: expected either %v or %v, got %v", "specified conflicting plugin configs 01", KubeAdmissionPlugins, openshiftAdmissionControlPlugins, pluginNames)
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
				if !reflect.DeepEqual(pluginNames, CombinedAdmissionControlPlugins) {
					t.Errorf("%s: expected %v, got %v", "specified, non-conflicting plugin configs 01", CombinedAdmissionControlPlugins, pluginNames)
				}
				return nil, nil
			},
		},
	}

	for _, tc := range testCases {
		newAdmissionChainFunc = tc.admissionChainBuilder
		_, _ = NewAdmissionChains(tc.options.APIServerArguments["admission-control-config-file"], tc.options.AdmissionPluginConfig, nil, nil)
	}
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

	combinedLen := len(CombinedAdmissionControlPlugins)
	if CombinedAdmissionControlPlugins[combinedLen-2] != "ResourceQuota" {
		t.Errorf("combinedAdmissionControlPlugins must have %s as the next to last plugin", "ResourceQuota")
	}

	if CombinedAdmissionControlPlugins[combinedLen-1] != "openshift.io/ClusterResourceQuota" {
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
