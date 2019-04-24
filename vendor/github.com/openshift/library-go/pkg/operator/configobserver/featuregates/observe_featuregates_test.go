package featuregates

import (
	"reflect"
	"testing"

	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	configv1 "github.com/openshift/api/config/v1"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/library-go/pkg/operator/events"
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

		configValue    configv1.FeatureSet
		expectedResult []string
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
				"CSIBlockVolume=true",
				"LocalStorageCapacityIsolation=false",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			indexer.Add(&configv1.FeatureGate{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: configv1.FeatureGateSpec{
					FeatureSet: tc.configValue,
				},
			})
			listers := testLister{
				lister: configlistersv1.NewFeatureGateLister(indexer),
			}
			eventRecorder := events.NewInMemoryRecorder("")

			initialExistingConfig := map[string]interface{}{}

			observeFn := NewObserveFeatureFlagsFunc(nil, configPath)

			observed, errs := observeFn(listers, eventRecorder, initialExistingConfig)
			if len(errs) != 0 {
				t.Fatal(errs)
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
