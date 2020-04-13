package apiserver

import (
	"reflect"
	"sort"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

func TestObserveAdditionalCORSAllowedOrigins(t *testing.T) {
	tests := []struct {
		name           string
		existingConfig map[string]interface{}
		corsConfig     []string
		expectedCORS   []string
	}{
		{
			name:           "emtpy",
			existingConfig: map[string]interface{}{},
			expectedCORS:   clusterDefaultCORSAllowedOrigins,
		},
		{
			name:           "some config",
			existingConfig: map[string]interface{}{},
			corsConfig:     []string{"//10.0.2.15:8443", `//https:\/\/myspecialsite.com:443$`},
			expectedCORS:   sortedStringSlice(append(clusterDefaultCORSAllowedOrigins, "//10.0.2.15:8443", `//https:\/\/myspecialsite.com:443$`)),
		},
		{
			name: "replace old config",
			existingConfig: map[string]interface{}{
				"corsAllowedOrigins": []interface{}{
					`//localhost(:|$)`,
					`//https:\/\/myspecialsite.com:443$`,
				},
			},
			corsConfig:   []string{`//https:\/\/myspecialsite.com:443$`},
			expectedCORS: sortedStringSlice(append(clusterDefaultCORSAllowedOrigins, `//https:\/\/myspecialsite.com:443$`)),
		},
	}
	for _, tt := range tests {
		for _, useAPIServerArguments := range []bool{false, true} {
			var corsPath []string
			if useAPIServerArguments {
				corsPath = []string{"apiServerArguments", "cors-allowed-origins"}
			} else {
				corsPath = []string{"corsAllowedOrigins"}
			}
			t.Run(tt.name, func(t *testing.T) {
				indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
				if tt.corsConfig != nil {
					if err := indexer.Add(&configv1.APIServer{
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster",
						},
						Spec: configv1.APIServerSpec{
							AdditionalCORSAllowedOrigins: tt.corsConfig,
						},
					}); err != nil {
						t.Fatal(err)
					}
				}
				listers := testLister{
					lister: configlistersv1.NewAPIServerLister(indexer),
				}

				var gotConfig map[string]interface{}
				var errs []error
				if useAPIServerArguments {
					gotConfig, errs = ObserveAdditionalCORSAllowedOriginsToArguments(listers, events.NewInMemoryRecorder(t.Name()), tt.existingConfig)
				} else {
					gotConfig, errs = ObserveAdditionalCORSAllowedOrigins(listers, events.NewInMemoryRecorder(t.Name()), tt.existingConfig)
				}
				if len(errs) > 0 {
					t.Errorf("expected no errors, got %v", errs)
				}

				gotCors, _, _ := unstructured.NestedStringSlice(gotConfig, corsPath...)
				if !reflect.DeepEqual(gotCors, tt.expectedCORS) {
					t.Fatalf("got = %v, want %v", gotCors, tt.expectedCORS)
				}

			})
		}
	}
}

func sortedStringSlice(ss []string) []string {
	sortable := sort.StringSlice(ss)
	sortable.Sort()
	return sortable
}
