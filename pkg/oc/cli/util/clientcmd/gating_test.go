package clientcmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	restclient "k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// TestDiscoveryResourceGate tests the legacy policy gate and GVR resource discovery
func TestDiscoveryResourceGate(t *testing.T) {
	resources := map[string][]metav1.APIResource{
		"allLegacy": {
			{Name: "clusterpolicies", Kind: "ClusterPolicies"},
			{Name: "clusterpolicybindings", Kind: "ClusterPolicyBindings"},
			{Name: "policies", Kind: "Policies"},
			{Name: "policybindings", Kind: "PolicyBindings"},
			{Name: "foo", Kind: "Foo"},
		},
		"partialLegacy": {
			{Name: "clusterpolicies", Kind: "ClusterPolicies"},
			{Name: "clusterpolicybindings", Kind: "ClusterPolicyBindings"},
			{Name: "foo", Kind: "Foo"},
		},
		"noLegacy": {
			{Name: "foo", Kind: "Foo"},
			{Name: "bar", Kind: "Bar"},
		},
	}

	legacyTests := map[string]struct {
		existingResources *metav1.APIResourceList
		expectErrStr      string
	}{
		"scheme-legacy-all-supported": {
			existingResources: &metav1.APIResourceList{
				GroupVersion: authorization.LegacySchemeGroupVersion.String(),
				APIResources: resources["allLegacy"],
			},
			expectErrStr: "",
		},
		"scheme-legacy-some-supported": {
			existingResources: &metav1.APIResourceList{
				GroupVersion: authorization.LegacySchemeGroupVersion.String(),
				APIResources: resources["partialLegacy"],
			},
			expectErrStr: "the server does not support legacy policy resources",
		},
		"scheme-legacy-none-supported": {
			existingResources: &metav1.APIResourceList{
				GroupVersion: authorization.LegacySchemeGroupVersion.String(),
				APIResources: resources["noLegacy"],
			},
			expectErrStr: "the server does not support legacy policy resources",
		},
		"scheme-all-supported": {
			existingResources: &metav1.APIResourceList{
				GroupVersion: authorization.SchemeGroupVersion.String(),
				APIResources: resources["allLegacy"],
			},
			expectErrStr: "",
		},
		"scheme-some-supported": {
			existingResources: &metav1.APIResourceList{
				GroupVersion: authorization.SchemeGroupVersion.String(),
				APIResources: resources["partialLegacy"],
			},
			expectErrStr: "the server does not support legacy policy resources",
		},
		"scheme-none-supported": {
			existingResources: &metav1.APIResourceList{
				GroupVersion: authorization.SchemeGroupVersion.String(),
				APIResources: resources["noLegacy"],
			},
			expectErrStr: "the server does not support legacy policy resources",
		},
	}

	discoveryTests := map[string]struct {
		existingResources *metav1.APIResourceList
		inputGVR          []schema.GroupVersionResource
		expectedGVR       []schema.GroupVersionResource
		expectedAll       bool
	}{
		"discovery-subset": {
			existingResources: &metav1.APIResourceList{
				GroupVersion: "v1",
				APIResources: resources["noLegacy"],
			},
			inputGVR: []schema.GroupVersionResource{
				{
					Group:    "",
					Version:  "v1",
					Resource: "foo",
				},
				{
					Group:    "",
					Version:  "v1",
					Resource: "bar",
				},
				{
					Group:    "",
					Version:  "v1",
					Resource: "noexist",
				},
			},
			expectedGVR: []schema.GroupVersionResource{
				{
					Group:    "",
					Version:  "v1",
					Resource: "foo",
				},
				{
					Group:    "",
					Version:  "v1",
					Resource: "bar",
				},
			},
		},
		"discovery-none": {
			existingResources: &metav1.APIResourceList{
				GroupVersion: "v1",
				APIResources: resources["noLegacy"],
			},
			inputGVR: []schema.GroupVersionResource{
				{
					Group:    "",
					Version:  "v1",
					Resource: "noexist",
				},
			},
			expectedGVR: []schema.GroupVersionResource{},
		},
		"discovery-all": {
			existingResources: &metav1.APIResourceList{
				GroupVersion: "v1",
				APIResources: resources["noLegacy"],
			},
			inputGVR: []schema.GroupVersionResource{
				{
					Group:    "",
					Version:  "v1",
					Resource: "foo",
				},
				{
					Group:    "",
					Version:  "v1",
					Resource: "bar",
				},
			},
			expectedGVR: []schema.GroupVersionResource{
				{
					Group:    "",
					Version:  "v1",
					Resource: "foo",
				},
				{
					Group:    "",
					Version:  "v1",
					Resource: "bar",
				},
			},
			expectedAll: true,
		},
	}

	for tcName, tc := range discoveryTests {
		func() {
			server := testServer(t, tc.existingResources)
			defer server.Close()
			client := discovery.NewDiscoveryClientForConfigOrDie(&restclient.Config{Host: server.URL})

			got, all, err := DiscoverGroupVersionResources(client, tc.inputGVR...)
			if err != nil {
				t.Fatalf("myerr %s", err.Error())
			}
			if !reflect.DeepEqual(got, tc.expectedGVR) {
				t.Fatalf("%s got %v, expected %v", tcName, got, tc.expectedGVR)
			}
			if tc.expectedAll && !all {
				t.Fatalf("%s expected all", tcName)
			}
		}()
	}

	for tcName, tc := range legacyTests {
		func() {
			server := testServer(t, tc.existingResources)
			defer server.Close()
			client := discovery.NewDiscoveryClientForConfigOrDie(&restclient.Config{Host: server.URL})

			err := LegacyPolicyResourceGate(client)
			if err != nil {
				if len(tc.expectErrStr) == 0 {
					t.Fatalf("%s unexpected err %s\n", tcName, err.Error())
				}
				if tc.expectErrStr != err.Error() {
					t.Fatalf("%s expected err %s, got %s", tcName, tc.expectErrStr, err.Error())
				}
			}
			if err == nil && len(tc.expectErrStr) != 0 {
				t.Fatalf("%s expected err %s, got none\n", tcName, tc.expectErrStr)
			}
		}()
	}
}

func testServer(t *testing.T, inputList *metav1.APIResourceList) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var list interface{}
		switch req.URL.Path {
		case "/apis/" + authorization.LegacySchemeGroupVersion.String():
			list = inputList
		case "/apis/" + authorization.SchemeGroupVersion.String():
			list = inputList
		case "/api/v1":
			list = inputList
		default:
			t.Logf("unexpected request: %s", req.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		output, err := json.Marshal(list)
		if err != nil {
			t.Errorf("unexpected encoding error: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(output)
	}))
}

// TestCountResourceDiscoveryCache tests the DiscoverGroupVersionResources() in-function GroupVersion cache.
func TestCountResourceDiscoveryCache(t *testing.T) {
	discoveryTests := map[string]struct {
		inputGVR     []schema.GroupVersionResource
		expectedGVR  []schema.GroupVersionResource
		expectedHits int
	}{
		"discovery-cache-gv": {
			inputGVR: []schema.GroupVersionResource{
				{
					Group:    "",
					Version:  "foobar",
					Resource: "foo",
				},
				{
					Group:    "",
					Version:  "foobar",
					Resource: "bar",
				},
				{
					Group:    "",
					Version:  "foobar",
					Resource: "baz",
				},
			},
			expectedGVR: []schema.GroupVersionResource{
				{
					Group:    "",
					Version:  "foobar",
					Resource: "foo",
				},
				{
					Group:    "",
					Version:  "foobar",
					Resource: "bar",
				},
			},
			expectedHits: 1,
		},
		"discovery-cache-separate-gv": {
			inputGVR: []schema.GroupVersionResource{
				{
					Group:    "",
					Version:  "foobar",
					Resource: "foo",
				},
				{
					Group:    "",
					Version:  "foobar",
					Resource: "bar",
				},
				{
					Group:    "",
					Version:  "foobar2",
					Resource: "baz",
				},
			},
			expectedGVR: []schema.GroupVersionResource{
				{
					Group:    "",
					Version:  "foobar",
					Resource: "foo",
				},
				{
					Group:    "",
					Version:  "foobar",
					Resource: "bar",
				},
			},
			expectedHits: 2,
		},
	}

	for tcName, tc := range discoveryTests {
		client := &countDiscoveryClient{}
		got, _, err := DiscoverGroupVersionResources(client, tc.inputGVR...)
		if err != nil {
			t.Fatalf("myerr %s", err.Error())
		}
		if tc.expectedHits != client.hits {
			t.Fatalf("%s wrong number of round trips, expected %v, got %v", tcName, tc.expectedHits, client.hits)
		}
		if !reflect.DeepEqual(got, tc.expectedGVR) {
			t.Fatalf("%s got %v, expected %v", tcName, got, tc.expectedGVR)
		}
	}
}

type countDiscoveryClient struct {
	hits int
}

func (c *countDiscoveryClient) ServerResourcesForGroupVersion(groupVersion string) (resources *metav1.APIResourceList, err error) {
	if groupVersion == "foobar" || groupVersion == "foobar2" {
		c.hits++
		return &metav1.APIResourceList{
			GroupVersion: groupVersion,
			APIResources: []metav1.APIResource{
				{
					Name: "foo",
				},
				{
					Name: "bar",
				},
			},
		}, nil
	}
	return nil, nil
}

func (c *countDiscoveryClient) ServerResources() ([]*metav1.APIResourceList, error) {
	panic("Unexpected call to ServerResources() in mock implementation")
}
func (c *countDiscoveryClient) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	panic("Unexpected call to ServerPreferredResources() in mock implementation")
}
func (c *countDiscoveryClient) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	panic("Unexpected call to ServerPreferredNamespacedResources() in mock implementation")
}
