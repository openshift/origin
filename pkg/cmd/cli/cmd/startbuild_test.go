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

	client "k8s.io/kubernetes/pkg/client/unversioned"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type FakeClientConfig struct {
	Raw      clientcmdapi.Config
	Client   *client.Config
	NS       string
	Explicit bool
	Err      error
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
	f := clientcmd.NewFactory(cfg)
	buf := &bytes.Buffer{}
	if err := RunStartBuildWebHook(f, buf, server.URL+"/webhook", "", "", nil); err != nil {
		t.Fatalf("unable to start hook: %v", err)
	}
	<-invoked

	if err := RunStartBuildWebHook(f, buf, server.URL+"/webhook", "", "unknownpath", nil); err == nil {
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
	if err := RunStartBuildWebHook(f, buf, server.URL+"/webhook", "", "", nil); err == nil || !strings.Contains(err.Error(), "certificate signed by unknown authority") {
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
	factory := clientcmd.NewFactory(cfg)
	buf := &bytes.Buffer{}
	if err := RunStartBuildWebHook(factory, buf, server.URL+"/webhook", "", f.Name(), nil); err != nil {
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
