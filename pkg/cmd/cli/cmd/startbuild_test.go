package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

type FakeClientConfig struct {
	Raw      clientcmdapi.Config
	Client   *restclient.Config
	NS       string
	Explicit bool
	Err      error
}

func (c *FakeClientConfig) ConfigAccess() kclientcmd.ConfigAccess {
	return nil
}

// RawConfig returns the merged result of all overrides
func (c *FakeClientConfig) RawConfig() (clientcmdapi.Config, error) {
	return c.Raw, c.Err
}

// ClientConfig returns a complete client config
func (c *FakeClientConfig) ClientConfig() (*restclient.Config, error) {
	return c.Client, c.Err
}

// Namespace returns the namespace resulting from the merged result of all overrides
func (c *FakeClientConfig) Namespace() (string, bool, error) {
	return c.NS, c.Explicit, c.Err
}

func TestStartBuildWebHook(t *testing.T) {
	invoked := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		invoked <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &FakeClientConfig{}
	buf := &bytes.Buffer{}
	o := &StartBuildOptions{
		Out:          buf,
		ClientConfig: cfg,
		FromWebhook:  server.URL + "/webhook",
		Mapper:       registered.RESTMapper(),
	}
	if err := o.Run(); err != nil {
		t.Fatalf("unable to start hook: %v", err)
	}
	<-invoked

	o = &StartBuildOptions{
		Out:            buf,
		FromWebhook:    server.URL + "/webhook",
		GitPostReceive: "unknownpath",
	}
	if err := o.Run(); err == nil {
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
	buf := &bytes.Buffer{}
	o := &StartBuildOptions{
		Out:          buf,
		ClientConfig: cfg,
		FromWebhook:  server.URL + "/webhook",
		Mapper:       registered.RESTMapper(),
	}
	if err := o.Run(); err == nil || !strings.Contains(err.Error(), "certificate signed by unknown authority") {
		t.Fatalf("unexpected non-error: %v", err)
	}
}

func TestStartBuildHookPostReceive(t *testing.T) {
	invoked := make(chan *buildapi.GenericWebHookEvent, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		event := buildapi.GenericWebHookEvent{}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&event); err != nil {
			t.Errorf("unmarshal failed: %v", err)
		}
		invoked <- &event
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	f, _ := ioutil.TempFile("", "test")
	defer os.Remove(f.Name())
	fmt.Fprintf(f, `0000 2384 refs/heads/master
2548 2548 refs/heads/stage`)
	f.Close()

	testErr := errors.New("not enabled")
	cfg := &FakeClientConfig{
		Err: testErr,
	}
	buf := &bytes.Buffer{}
	o := &StartBuildOptions{
		Out:            buf,
		ClientConfig:   cfg,
		FromWebhook:    server.URL + "/webhook",
		GitPostReceive: f.Name(),
		Mapper:         registered.RESTMapper(),
	}
	if err := o.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := <-invoked
	if event == nil || event.Git == nil || len(event.Git.Refs) != 1 {
		t.Fatalf("unexpected event: %#v", event)
	}
	if event.Git.Refs[0].Commit != "2384" {
		t.Fatalf("unexpected ref: %#v", event.Git.Refs[0])
	}
}
