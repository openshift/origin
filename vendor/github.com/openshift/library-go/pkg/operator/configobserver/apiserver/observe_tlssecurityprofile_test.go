package apiserver

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	configv1 "github.com/openshift/api/config/v1"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
)

type testLister struct {
	lister configlistersv1.APIServerLister
}

func (l testLister) APIServerLister() configlistersv1.APIServerLister {
	return l.lister
}

func (l testLister) ResourceSyncer() resourcesynccontroller.ResourceSyncer {
	return nil
}

func (l testLister) PreRunHasSynced() []cache.InformerSynced {
	return nil
}
func TestObserveTLSSecurityProfile(t *testing.T) {
	existingConfig := map[string]interface{}{
		"minTLSVersion": "VersionTLS11",
		"cipherSuites":  []string{"DES-CBC3-SHA"},
	}

	tests := []struct {
		name                  string
		config                *configv1.TLSSecurityProfile
		existing              map[string]interface{}
		expectedMinTLSVersion string
		expectedSuites        []string
	}{
		{
			name:                  "NoAPIServerConfig",
			config:                nil,
			existing:              existingConfig,
			expectedMinTLSVersion: "VersionTLS12",
			expectedSuites:        crypto.OpenSSLToIANACipherSuites(configv1.TLSProfiles[configv1.TLSProfileIntermediateType].Ciphers),
		},
		{
			name: "ModernCrypto",
			config: &configv1.TLSSecurityProfile{
				Type:   configv1.TLSProfileModernType,
				Modern: &configv1.ModernTLSProfile{},
			},
			existing:              existingConfig,
			expectedMinTLSVersion: "VersionTLS13",
			expectedSuites:        crypto.OpenSSLToIANACipherSuites(configv1.TLSProfiles[configv1.TLSProfileModernType].Ciphers),
		},
		{
			name: "OldCrypto",
			config: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileOldType,
				Old:  &configv1.OldTLSProfile{},
			},
			existing:              existingConfig,
			expectedMinTLSVersion: "VersionTLS10",
			expectedSuites:        crypto.OpenSSLToIANACipherSuites(configv1.TLSProfiles[configv1.TLSProfileOldType].Ciphers),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			if tt.config != nil {
				if err := indexer.Add(&configv1.APIServer{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: configv1.APIServerSpec{
						TLSSecurityProfile: tt.config,
					},
				}); err != nil {
					t.Fatal(err)
				}
			}
			listers := testLister{
				lister: configlistersv1.NewAPIServerLister(indexer),
			}

			result, errs := ObserveTLSSecurityProfile(listers, events.NewInMemoryRecorder(t.Name()), tt.existing)
			if len(errs) > 0 {
				t.Errorf("expected 0 errors, got %v", errs)
			}

			gotMinTLSVersion, _, err := unstructured.NestedString(result, "servingInfo", "minTLSVersion")
			if err != nil {
				t.Errorf("couldn't get minTLSVersion from the returned object: %v", err)
			}

			gotSuites, _, err := unstructured.NestedStringSlice(result, "servingInfo", "cipherSuites")
			if err != nil {
				t.Errorf("couldn't get cipherSuites from the returned object: %v", err)
			}

			if !reflect.DeepEqual(gotSuites, tt.expectedSuites) {
				t.Errorf("ObserveTLSSecurityProfile() got cipherSuites = %v, expected %v", gotSuites, tt.expectedSuites)
			}
			if gotMinTLSVersion != tt.expectedMinTLSVersion {
				t.Errorf("ObserveTLSSecurityProfile() got minTlSVersion = %v, expected %v", gotMinTLSVersion, tt.expectedMinTLSVersion)
			}
		})
	}
}
