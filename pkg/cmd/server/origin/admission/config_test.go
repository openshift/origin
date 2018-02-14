package admission

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/apis/clusterresourceoverride"
	sccadmission "github.com/openshift/origin/pkg/security/admission"
	serviceadmit "github.com/openshift/origin/pkg/service/admission"
)

// TestAdmissionPluginChains makes sure that the admission plugin lists are coherent.
// we have to maintain three different lists of plugins: default origin, default kube, default combined
// the set of (default origin and default kube) and default combined, but must be equal
// the order of default origin must follow the order of default combined
// the order of default kube must follow the order of default combined
func TestAdmissionPluginChains(t *testing.T) {
	individualSet := sets.NewString(openshiftAdmissionControlPlugins...)
	individualSet.Insert(kubeAdmissionPlugins...)
	combinedSet := sets.NewString(combinedAdmissionControlPlugins...)

	if !individualSet.Equal(combinedSet) {
		t.Fatalf("individualSets are missing: %v combinedSet is missing: %v", combinedSet.Difference(individualSet), individualSet.Difference(combinedSet))
	}

	lastCurrIndex := -1
	for _, plugin := range openshiftAdmissionControlPlugins {
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
	for _, plugin := range kubeAdmissionPlugins {
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
	"OriginNamespaceLifecycle",
	"PodNodeConstraints",
	"BuildByStrategy",
	"RunOnceDuration",
	"OriginPodNodeEnvironment",
	overrideapi.PluginName,
	serviceadmit.ExternalIPPluginName,
	sccadmission.PluginName,
	"SCCExecRestrictions",
	"ResourceQuota",
)

// TestAdmissionPluginNames makes sure that openshift admission plugins are prefixed with `openshift.io/`.
func TestAdmissionPluginNames(t *testing.T) {
	originAdmissionPlugins := admission.NewPlugins()
	registerOpenshiftAdmissionPlugins(originAdmissionPlugins)

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
		options               configapi.MasterConfig
		admissionChainBuilder func(pluginNames []string, admissionConfigFilename string, options configapi.MasterConfig, pluginInitializer admission.PluginInitializer, decorator admission.Decorator) (admission.Interface, error)
	}{
		{
			name: "stock everything",
			options: configapi.MasterConfig{
				KubernetesMasterConfig: &configapi.KubernetesMasterConfig{},
			},
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, options configapi.MasterConfig, pluginInitializer admission.PluginInitializer, decorator admission.Decorator) (admission.Interface, error) {
				if !reflect.DeepEqual(pluginNames, combinedAdmissionControlPlugins) {
					t.Errorf("%s: expected %v, got %v", "stock everything", combinedAdmissionControlPlugins, pluginNames)
				}
				return nil, nil
			},
		},
		{
			name: "specified origin admission order",
			options: configapi.MasterConfig{
				KubernetesMasterConfig: &configapi.KubernetesMasterConfig{},
				AdmissionConfig: configapi.AdmissionConfig{
					PluginOrderOverride: []string{"foo"},
				},
			},
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, options configapi.MasterConfig, pluginInitializer admission.PluginInitializer, decorator admission.Decorator) (admission.Interface, error) {
				isKube := reflect.DeepEqual(pluginNames, combinedAdmissionControlPlugins)

				expectedOrigin := []string{"foo"}
				isOrigin := reflect.DeepEqual(pluginNames, expectedOrigin)

				if !isKube && !isOrigin {
					t.Errorf("%s: expected either %v or %v, got %v", "specified origin admission order", kubeAdmissionPlugins, expectedOrigin, pluginNames)
				}

				return nil, nil
			},
		},
		{
			name: "specified kube admission config file",
			options: configapi.MasterConfig{
				KubernetesMasterConfig: &configapi.KubernetesMasterConfig{
					APIServerArguments: configapi.ExtendedArguments{
						"admission-control-config-file": []string{"foo"},
					},
				},
			},
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, options configapi.MasterConfig, pluginInitializer admission.PluginInitializer, decorator admission.Decorator) (admission.Interface, error) {
				isKube := reflect.DeepEqual(pluginNames, combinedAdmissionControlPlugins)
				isOrigin := reflect.DeepEqual(pluginNames, combinedAdmissionControlPlugins)
				if !isKube && !isOrigin {
					t.Errorf("%s: expected either %v or %v, got %v", "specified conflicting plugin configs 01", kubeAdmissionPlugins, openshiftAdmissionControlPlugins, pluginNames)
				}
				return nil, nil
			},
		},
		{
			name: "specified, non-conflicting plugin configs 01",
			options: configapi.MasterConfig{
				KubernetesMasterConfig: &configapi.KubernetesMasterConfig{},
				AdmissionConfig: configapi.AdmissionConfig{
					PluginConfig: map[string]*configapi.AdmissionPluginConfig{
						"foo": {
							Location: "bar",
						},
					},
				},
			},
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, options configapi.MasterConfig, pluginInitializer admission.PluginInitializer, decorator admission.Decorator) (admission.Interface, error) {
				if !reflect.DeepEqual(pluginNames, combinedAdmissionControlPlugins) {
					t.Errorf("%s: expected %v, got %v", "specified, non-conflicting plugin configs 01", combinedAdmissionControlPlugins, pluginNames)
				}
				return nil, nil
			},
		},
	}

	for _, tc := range testCases {
		newAdmissionChainFunc = tc.admissionChainBuilder
		_, _ = NewAdmissionChains(tc.options, nil, nil)
	}
}

func TestQuotaAdmissionPluginsAreLast(t *testing.T) {
	kubeLen := len(kubeAdmissionPlugins)
	if kubeLen < 2 {
		t.Fatalf("must have at least the 2 quota plugins")
	}

	if kubeAdmissionPlugins[kubeLen-2] != "ResourceQuota" {
		t.Errorf("kubeAdmissionPlugins must have %s as the next to last plugin", "ResourceQuota")
	}

	if kubeAdmissionPlugins[kubeLen-1] != "openshift.io/ClusterResourceQuota" {
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
