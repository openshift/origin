package clientcmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/kubernetes/pkg/client/restclient"

	"github.com/openshift/origin/pkg/api/v1"
	"github.com/openshift/origin/pkg/api/v1beta3"
	"github.com/openshift/origin/pkg/client"
)

func TestClientConfigForVersion(t *testing.T) {
	called := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/oapi" {
			t.Errorf("Unexpected path called during negotiation: %s", req.URL.Path)
			return
		}
		called++
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

	// Call for unsupported version, negotiate to supported version
	called = 0
	v1beta3Config, err := clients.ClientConfigForVersion(&v1beta3.SchemeGroupVersion)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if v1beta3Config.GroupVersion.String() != "v1" {
		t.Fatalf("Expected to negotiate v1 for v1beta3 config, got %v", v1beta3Config.GroupVersion.String())
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
