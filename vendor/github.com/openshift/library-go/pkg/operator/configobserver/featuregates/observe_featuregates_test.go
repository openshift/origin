package featuregates

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	configv1 "github.com/openshift/api/config/v1"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
)

type testLister struct {
	lister configlistersv1.FeatureGateLister
}

func (l testLister) FeatureGateLister() configlistersv1.FeatureGateLister {
	return l.lister
}

func (l testLister) ResourceSyncer() resourcesynccontroller.ResourceSyncer {
	return nil
}

func (l testLister) PreRunHasSynced() []cache.InformerSynced {
	return nil
}

func TestObserveFeatureFlags(t *testing.T) {
	configPath := []string{"foo", "bar"}

	tests := []struct {
		name string

		configValue     configv1.FeatureSet
		expectedResult  []string
		expectError     bool
		customNoUpgrade *configv1.CustomFeatureGates
		knownFeatures   sets.String
	}{
		{
			name:        "default",
			configValue: configv1.Default,
			expectedResult: []string{
				"ExperimentalCriticalPodAnnotation=true",
				"RotateKubeletServerCertificate=true",
				"SupportPodPidsLimit=true",
				"LocalStorageCapacityIsolation=false",
			},
		},
		{
			name:        "techpreview",
			configValue: configv1.TechPreviewNoUpgrade,
			expectedResult: []string{
				"ExperimentalCriticalPodAnnotation=true",
				"RotateKubeletServerCertificate=true",
				"SupportPodPidsLimit=true",
				"LocalStorageCapacityIsolation=false",
			},
		},
		{
			name:        "custom no upgrade and all allowed",
			configValue: configv1.CustomNoUpgrade,
			expectedResult: []string{
				"CustomFeatureEnabled=true",
				"CustomFeatureDisabled=false",
			},
			customNoUpgrade: &configv1.CustomFeatureGates{
				Enabled:  []string{"CustomFeatureEnabled"},
				Disabled: []string{"CustomFeatureDisabled"},
			},
		},
		{
			name:           "custom no upgrade flag set and none upgrades were provided",
			configValue:    configv1.CustomNoUpgrade,
			expectedResult: []string{},
		},
		{
			name:        "custom no upgrade and known features",
			configValue: configv1.CustomNoUpgrade,
			expectedResult: []string{
				"CustomFeatureEnabled=true",
			},
			customNoUpgrade: &configv1.CustomFeatureGates{
				Enabled:  []string{"CustomFeatureEnabled"},
				Disabled: []string{"CustomFeatureDisabled"},
			},
			knownFeatures: sets.NewString("CustomFeatureEnabled"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			indexer.Add(&configv1.FeatureGate{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: configv1.FeatureGateSpec{
					FeatureGateSelection: configv1.FeatureGateSelection{
						FeatureSet:      tc.configValue,
						CustomNoUpgrade: tc.customNoUpgrade,
					},
				},
			})
			listers := testLister{
				lister: configlistersv1.NewFeatureGateLister(indexer),
			}
			eventRecorder := events.NewInMemoryRecorder("")

			initialExistingConfig := map[string]interface{}{}

			observeFn := NewObserveFeatureFlagsFunc(tc.knownFeatures, configPath)

			observed, errs := observeFn(listers, eventRecorder, initialExistingConfig)
			if len(errs) != 0 && !tc.expectError {
				t.Fatal(errs)
			}
			if len(errs) == 0 && tc.expectError {
				t.Fatal("expected an error but got nothing")
			}
			actual, _, err := unstructured.NestedStringSlice(observed, configPath...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(tc.expectedResult, actual) {
				t.Errorf("%v", actual)
			}
		})
	}
}
