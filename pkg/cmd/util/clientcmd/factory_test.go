package clientcmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	kclient "k8s.io/kubernetes/pkg/client/unversioned"

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

	defaultConfig := &kclient.Config{Host: server.URL}
	client.SetOpenShiftDefaults(defaultConfig)

	clients := &clientCache{
		clients:       make(map[string]*client.Client),
		configs:       make(map[string]*kclient.Config),
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
