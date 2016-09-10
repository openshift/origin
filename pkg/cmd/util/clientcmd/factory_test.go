package clientcmd

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/api/v1"
	"github.com/openshift/origin/pkg/client"
)

// TestRunGenerators makes sure we catch new generators added to `oc run`
func TestRunGenerators(t *testing.T) {
	f := NewFactory(nil)

	// Contains the run generators we expect to see
	expectedRunGenerators := sets.NewString(
		// kube generators
		"run/v1",
		"run-pod/v1",
		"deployment/v1beta1",
		"job/v1",
		"job/v1beta1",
		"scheduledjob/v2alpha1",

		// origin generators
		"run-controller/v1", // legacy alias for run/v1
		"deploymentconfig/v1",
	).List()

	runGenerators := sets.StringKeySet(f.Generators("run")).List()
	if !reflect.DeepEqual(expectedRunGenerators, runGenerators) {
		t.Errorf("Expected run generators:%#v, got:\n%#v", expectedRunGenerators, runGenerators)
	}
}

func TestClientConfigForVersion(t *testing.T) {
	called := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/oapi" {
			t.Errorf("Unexpected path called during negotiation: %s", req.URL.Path)
			return
		}
		called++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"versions":["v1"]}`))
	}))
	defer server.Close()

	defaultConfig := &restclient.Config{Host: server.URL}
	client.SetOpenShiftDefaults(defaultConfig)

	clients := &clientCache{
		clients:       make(map[string]*client.Client),
		configs:       make(map[string]*restclient.Config),
		defaultConfig: defaultConfig,
	}

	// First call, negotiate
	called = 0
	v1Config, err := clients.ClientConfigForVersion(nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if v1Config.GroupVersion.String() != "v1" {
		t.Fatalf("Expected v1, got %v", v1Config.GroupVersion.String())
	}
	if called != 1 {
		t.Fatalf("Expected to be called 1 time during negotiation, was called %d times", called)
	}

	// Second call, cache
	called = 0
	v1Config, err = clients.ClientConfigForVersion(nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if v1Config.GroupVersion.String() != "v1" {
		t.Fatalf("Expected v1, got %v", v1Config.GroupVersion.String())
	}
	if called != 0 {
		t.Fatalf("Expected not be called again getting a config from cache, was called %d additional times", called)
	}

	// Third call, cached under exactly matching version
	called = 0
	v1Config, err = clients.ClientConfigForVersion(&v1.SchemeGroupVersion)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if v1Config.GroupVersion.String() != "v1" {
		t.Fatalf("Expected v1, got %v", v1Config.GroupVersion.String())
	}
	if called != 0 {
		t.Fatalf("Expected not be called again getting a config from cache, was called %d additional times", called)
	}

	// Call for removed version, return error
	called = 0
	if _, err := clients.ClientConfigForVersion(&unversioned.GroupVersion{Version: "v1beta3"}); err == nil {
		t.Fatalf("Unexpected non-error: %v", err)
	}
	if called != 1 {
		t.Fatalf("Expected to be called once getting a config for a new version, was called %d times", called)
	}
}

func TestComputeDiscoverCacheDir(t *testing.T) {
	testCases := []struct {
		name      string
		parentDir string
		host      string

		expected string
	}{
		{
			name:      "simple append",
			parentDir: "~/",
			host:      "localhost:8443",
			expected:  "~/localhost_8443",
		},
		{
			name:      "with path",
			parentDir: "~/",
			host:      "localhost:8443/prefix",
			expected:  "~/localhost_8443/prefix",
		},
		{
			name:      "dotted name",
			parentDir: "~/",
			host:      "mine.example.org:8443",
			expected:  "~/mine.example.org_8443",
		},
		{
			name:      "IP",
			parentDir: "~/",
			host:      "127.0.0.1:8443",
			expected:  "~/127.0.0.1_8443",
		},
		{
			// restricted characters from: https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx#naming_conventions
			// it's not a complete list because they have a very helpful: "Any other character that the target file system does not allow."
			name:      "windows safe",
			parentDir: "~/",
			host:      `<>:"\|?*`,
			expected:  "~/________",
		},
	}

	for _, tc := range testCases {
		actual := computeDiscoverCacheDir(tc.parentDir, tc.host)
		if actual != tc.expected {
			t.Errorf("%s: expected %v, got %v", tc.name, tc.expected, actual)
		}
	}
}
