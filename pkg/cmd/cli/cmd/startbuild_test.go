package cmd

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type FakeClientConfig struct {
	Raw    clientcmdapi.Config
	Client *client.Config
	NS     string
	Err    error
}

// RawConfig returns the merged result of all overrides
func (c *FakeClientConfig) RawConfig() (clientcmdapi.Config, error) {
	return c.Raw, c.Err
}

// ClientConfig returns a complete client config
func (c *FakeClientConfig) ClientConfig() (*client.Config, error) {
	return c.Client, c.Err
}

// Namespace returns the namespace resulting from the merged result of all overrides
func (c *FakeClientConfig) Namespace() (string, error) {
	return c.NS, c.Err
}

func TestStartBuildWebHook(t *testing.T) {
	invoked := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		invoked <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &FakeClientConfig{}
	f := clientcmd.NewFactory(cfg)
	buf := &bytes.Buffer{}
	if err := RunStartBuildWebHook(f, buf, server.URL+"/webhook", ""); err != nil {
		t.Fatalf("unable to start hook: %v", err)
	}
	<-invoked

	if err := RunStartBuildWebHook(f, buf, server.URL+"/webhook", "unknownpath"); err != nil {
		t.Fatalf("unexpected non-error: %v", err)
	}
}

func TestStartBuildWebHookHTTPS(t *testing.T) {
	invoked := make(chan struct{}, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		invoked <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	testErr := errors.New("not enabled")
	cfg := &FakeClientConfig{
		Err: testErr,
	}
	f := clientcmd.NewFactory(cfg)
	buf := &bytes.Buffer{}
	if err := RunStartBuildWebHook(f, buf, server.URL+"/webhook", ""); err == nil || !strings.Contains(err.Error(), "certificate signed by unknown authority") {
		t.Fatalf("unexpected non-error: %v", err)
	}
}
