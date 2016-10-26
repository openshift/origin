package origin

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/plugin/pkg/admission/namespace/lifecycle"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	serviceadmit "github.com/openshift/origin/pkg/service/admission"
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
	"OriginNamespaceLifecycle",
	"PodNodeConstraints",
	"BuildByStrategy",
	"RunOnceDuration",
	"OriginPodNodeEnvironment",
	overrideapi.PluginName,
	serviceadmit.ExternalIPPluginName,
	"SecurityContextConstraint",
	"SCCExecRestrictions",
)

// kubeAdmissionPlugins tracks kube plugins we use.  You may add to this list, but ONLY if they're from upstream kube
var kubeAdmissionPlugins = sets.NewString(
	lifecycle.PluginName,
	"LimitRanger",
	"ServiceAccount",
	"DefaultStorageClass",
	"ImagePolicyWebhook",
	"AlwaysPullImages",
	"LimitPodHardAntiAffinityTopology",
	"SCCExecRestrictions",
	"PersistentVolumeLabel",
	"OwnerReferencesPermissionEnforcement",
)

// TestAdmissionPluginNames makes sure that openshift admission plugins are prefixed with `openshift.io/`.
func TestAdmissionPluginNames(t *testing.T) {
	for _, plugin := range CombinedAdmissionControlPlugins {
		if !strings.HasPrefix(plugin, "openshift.io/") && !kubeAdmissionPlugins.Has(plugin) && !legacyOpenshiftAdmissionPlugins.Has(plugin) {
			t.Errorf("openshift admission plugins must be prefixed with openshift.io/ %v", plugin)
		}
	}
}

func TestSeparateAdmissionChainDetection(t *testing.T) {
	testCases := []struct {
		name                  string
		options               configapi.MasterConfig
		admissionChainBuilder func(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet *internalclientset.Clientset, pluginInitializer oadmission.PluginInitializer) (admission.Interface, error)
	}{
		{
			name: "stock everything",
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet *internalclientset.Clientset, pluginInitializer oadmission.PluginInitializer) (admission.Interface, error) {
				if !reflect.DeepEqual(pluginNames, CombinedAdmissionControlPlugins) {
					t.Errorf("%s: expected %v, got %v", "stock everything", CombinedAdmissionControlPlugins, pluginNames)
				}
				return nil, nil
			},
		},
		{
			name: "specified kube admission order 01",
			options: configapi.MasterConfig{
				KubernetesMasterConfig: &configapi.KubernetesMasterConfig{
					AdmissionConfig: configapi.AdmissionConfig{
						PluginOrderOverride: []string{"foo"},
					},
				},
			},
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet *internalclientset.Clientset, pluginInitializer oadmission.PluginInitializer) (admission.Interface, error) {
				expectedKube := []string{"foo"}
				isKube := reflect.DeepEqual(pluginNames, expectedKube)

				isOrigin := reflect.DeepEqual(pluginNames, openshiftAdmissionControlPlugins)

				if !isKube && !isOrigin {
					t.Errorf("%s: expected either %v or %v, got %v", "specified kube admission order 01", expectedKube, openshiftAdmissionControlPlugins, pluginNames)
				}

				return nil, nil
			},
		},
		{
			name: "specified kube admission order 02",
			options: configapi.MasterConfig{
				KubernetesMasterConfig: &configapi.KubernetesMasterConfig{
					APIServerArguments: configapi.ExtendedArguments{
						"admission-control": []string{"foo"},
					},
				},
			},
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet *internalclientset.Clientset, pluginInitializer oadmission.PluginInitializer) (admission.Interface, error) {
				expectedKube := []string{"foo"}
				isKube := reflect.DeepEqual(pluginNames, expectedKube)

				isOrigin := reflect.DeepEqual(pluginNames, openshiftAdmissionControlPlugins)

				if !isKube && !isOrigin {
					t.Errorf("%s: expected either %v or %v, got %v", "specified kube admission order 01", expectedKube, openshiftAdmissionControlPlugins, pluginNames)
				}

				return nil, nil
			},
		},
		{
			name: "specified origin admission order",
			options: configapi.MasterConfig{
				AdmissionConfig: configapi.AdmissionConfig{
					PluginOrderOverride: []string{"foo"},
				},
			},
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet *internalclientset.Clientset, pluginInitializer oadmission.PluginInitializer) (admission.Interface, error) {
				isKube := reflect.DeepEqual(pluginNames, KubeAdmissionPlugins)

				expectedOrigin := []string{"foo"}
				isOrigin := reflect.DeepEqual(pluginNames, expectedOrigin)

				if !isKube && !isOrigin {
					t.Errorf("%s: expected either %v or %v, got %v", "specified origin admission order", KubeAdmissionPlugins, expectedOrigin, pluginNames)
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
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet *internalclientset.Clientset, pluginInitializer oadmission.PluginInitializer) (admission.Interface, error) {
				isKube := reflect.DeepEqual(pluginNames, KubeAdmissionPlugins)
				isOrigin := reflect.DeepEqual(pluginNames, openshiftAdmissionControlPlugins)
				if !isKube && !isOrigin {
					t.Errorf("%s: expected either %v or %v, got %v", "specified conflicting plugin configs 01", KubeAdmissionPlugins, openshiftAdmissionControlPlugins, pluginNames)
				}
				return nil, nil
			},
		},
		{
			name: "specified, non-conflicting plugin configs 01",
			options: configapi.MasterConfig{
				AdmissionConfig: configapi.AdmissionConfig{
					PluginConfig: map[string]configapi.AdmissionPluginConfig{
						"foo": {
							Location: "bar",
						},
					},
				},
			},
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet *internalclientset.Clientset, pluginInitializer oadmission.PluginInitializer) (admission.Interface, error) {
				if !reflect.DeepEqual(pluginNames, CombinedAdmissionControlPlugins) {
					t.Errorf("%s: expected %v, got %v", "specified, non-conflicting plugin configs 01", CombinedAdmissionControlPlugins, pluginNames)
				}
				return nil, nil
			},
		},
		{
			name: "specified, non-conflicting plugin configs 02",
			options: configapi.MasterConfig{
				KubernetesMasterConfig: &configapi.KubernetesMasterConfig{
					AdmissionConfig: configapi.AdmissionConfig{
						PluginConfig: map[string]configapi.AdmissionPluginConfig{
							"foo": {
								Location: "bar",
							},
							"third": {
								Location: "bar",
							},
						},
					},
				},
				AdmissionConfig: configapi.AdmissionConfig{
					PluginConfig: map[string]configapi.AdmissionPluginConfig{
						"foo": {
							Location: "bar",
						},
					},
				},
			},
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet *internalclientset.Clientset, pluginInitializer oadmission.PluginInitializer) (admission.Interface, error) {
				if !reflect.DeepEqual(pluginNames, CombinedAdmissionControlPlugins) {
					t.Errorf("%s: expected %v, got %v", "specified, non-conflicting plugin configs 02", CombinedAdmissionControlPlugins, pluginNames)
				}
				return nil, nil
			},
		},
		{
			name: "specified, non-conflicting plugin configs 03",
			options: configapi.MasterConfig{
				KubernetesMasterConfig: &configapi.KubernetesMasterConfig{
					AdmissionConfig: configapi.AdmissionConfig{
						PluginConfig: map[string]configapi.AdmissionPluginConfig{
							"foo": {
								Location: "bar",
							},
							"third": {
								Location: "bar",
							},
						},
					},
				},
			},
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet *internalclientset.Clientset, pluginInitializer oadmission.PluginInitializer) (admission.Interface, error) {
				if !reflect.DeepEqual(pluginNames, CombinedAdmissionControlPlugins) {
					t.Errorf("%s: expected %v, got %v", "specified, non-conflicting plugin configs 03", CombinedAdmissionControlPlugins, pluginNames)
				}
				return nil, nil
			},
		},
		{
			name: "specified conflicting plugin configs 01",
			options: configapi.MasterConfig{
				KubernetesMasterConfig: &configapi.KubernetesMasterConfig{
					AdmissionConfig: configapi.AdmissionConfig{
						PluginConfig: map[string]configapi.AdmissionPluginConfig{
							"foo": {
								Location: "different",
							},
						},
					},
				},
				AdmissionConfig: configapi.AdmissionConfig{
					PluginConfig: map[string]configapi.AdmissionPluginConfig{
						"foo": {
							Location: "bar",
						},
					},
				},
			},
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet *internalclientset.Clientset, pluginInitializer oadmission.PluginInitializer) (admission.Interface, error) {
				isKube := reflect.DeepEqual(pluginNames, KubeAdmissionPlugins)
				isOrigin := reflect.DeepEqual(pluginNames, openshiftAdmissionControlPlugins)
				if !isKube && !isOrigin {
					t.Errorf("%s: expected either %v or %v, got %v", "specified conflicting plugin configs 01", KubeAdmissionPlugins, openshiftAdmissionControlPlugins, pluginNames)
				}

				return nil, nil
			},
		},
	}

	for _, tc := range testCases {
		newAdmissionChainFunc = tc.admissionChainBuilder
		_, _, _ = buildAdmissionChains(tc.options, nil, oadmission.PluginInitializer{})
	}
}
