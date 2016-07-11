package origin

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/util/sets"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

// TestAdmissionPluginChains makes sure that the admission plugin lists are coherent.
// we have to maintain three different lists of plugins: default origin, default kube, default combined
// the set of (default origin and default kube) and default combined, but must be equal
// the order of default origin must follow the order of default combined
// the order of default kube must follow the order of default combined
func TestAdmissionPluginChains(t *testing.T) {
	individualSet := sets.NewString(openshiftAdmissionControlPlugins...)
	individualSet.Insert(KubeAdmissionPlugins...)
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

func TestSeparateAdmissionChainDetection(t *testing.T) {
	testCases := []struct {
		name                  string
		options               configapi.MasterConfig
		admissionChainBuilder func(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet *internalclientset.Clientset, pluginInitializer oadmission.PluginInitializer) (admission.Interface, error)
	}{
		{
			name: "stock everything",
			admissionChainBuilder: func(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet *internalclientset.Clientset, pluginInitializer oadmission.PluginInitializer) (admission.Interface, error) {
				if !reflect.DeepEqual(pluginNames, combinedAdmissionControlPlugins) {
					t.Errorf("%s: expected %v, got %v", "stock everything", combinedAdmissionControlPlugins, pluginNames)
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
				if !reflect.DeepEqual(pluginNames, combinedAdmissionControlPlugins) {
					t.Errorf("%s: expected %v, got %v", "specified, non-conflicting plugin configs 01", combinedAdmissionControlPlugins, pluginNames)
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
				if !reflect.DeepEqual(pluginNames, combinedAdmissionControlPlugins) {
					t.Errorf("%s: expected %v, got %v", "specified, non-conflicting plugin configs 02", combinedAdmissionControlPlugins, pluginNames)
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
				if !reflect.DeepEqual(pluginNames, combinedAdmissionControlPlugins) {
					t.Errorf("%s: expected %v, got %v", "specified, non-conflicting plugin configs 03", combinedAdmissionControlPlugins, pluginNames)
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
