package proxy

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	configv1 "github.com/openshift/api/config/v1"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
)

type testLister struct {
	lister configlistersv1.ProxyLister
}

func (l testLister) ProxyLister() configlistersv1.ProxyLister {
	return l.lister
}

func (l testLister) ResourceSyncer() resourcesynccontroller.ResourceSyncer {
	return nil
}

func (l testLister) PreRunHasSynced() []cache.InformerSynced {
	return nil
}

func TestObserveProxyConfig(t *testing.T) {
	configPath := []string{"openshift", "proxy"}

	tests := []struct {
		name           string
		proxySpec      configv1.ProxySpec
		proxyStatus    configv1.ProxyStatus
		previous       map[string]string
		expected       map[string]interface{}
		expectedError  []error
		eventsExpected int
	}{
		{
			name:          "all unset",
			proxySpec:     configv1.ProxySpec{},
			proxyStatus:   configv1.ProxyStatus{},
			expected:      map[string]interface{}{},
			expectedError: []error{},
		},
		{
			name: "all set",
			proxySpec: configv1.ProxySpec{
				HTTPProxy:  "http://someplace.it",
				HTTPSProxy: "https://someplace.it",
				NoProxy:    "127.0.0.1",
			},
			proxyStatus: configv1.ProxyStatus{
				HTTPProxy:  "http://someplace.it",
				HTTPSProxy: "https://someplace.it",
				NoProxy:    "127.0.0.1,incluster.address.it",
			},
			expected: map[string]interface{}{
				"openshift": map[string]interface{}{
					"proxy": map[string]interface{}{
						"HTTP_PROXY":  "http://someplace.it",
						"HTTPS_PROXY": "https://someplace.it",
						"NO_PROXY":    "127.0.0.1,incluster.address.it",
					},
				},
			},
			expectedError:  []error{},
			eventsExpected: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			indexer.Add(&configv1.Proxy{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       tt.proxySpec,
				Status:     tt.proxyStatus,
			})
			listers := testLister{
				lister: configlistersv1.NewProxyLister(indexer),
			}
			eventRecorder := events.NewInMemoryRecorder("")

			initialExistingConfig := map[string]interface{}{}

			observeFn := NewProxyObserveFunc(configPath)

			got, errorsGot := observeFn(listers, eventRecorder, initialExistingConfig)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("observeProxyFlags.ObserveProxyConfig() got = %v, want %v", got, tt.expected)
			}
			if !reflect.DeepEqual(errorsGot, tt.expectedError) {
				t.Errorf("observeProxyFlags.ObserveProxyConfig() errorsGot = %v, want %v", errorsGot, tt.expectedError)
			}
			if events := eventRecorder.Events(); len(events) != tt.eventsExpected {
				t.Errorf("expected %d events, but got %d: %v", tt.eventsExpected, len(events), events)
			}
		})
	}
}
