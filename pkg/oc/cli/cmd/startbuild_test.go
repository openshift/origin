package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	restclient "k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
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
		Mapper:       legacyscheme.Registry.RESTMapper(),
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
		Mapper:       legacyscheme.Registry.RESTMapper(),
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
		Mapper:         legacyscheme.Registry.RESTMapper(),
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

type FakeBuildConfigs struct {
	t            *testing.T
	expectAsFile bool
}

func (c FakeBuildConfigs) InstantiateBinary(name string, options *buildapi.BinaryBuildRequestOptions, r io.Reader) (result *buildapi.Build, err error) {
	if binary, err := ioutil.ReadAll(r); err != nil {
		c.t.Errorf("Error while reading binary over HTTP: %v", err)
	} else if string(binary) != "hi" {
		c.t.Errorf("Wrong value while reading binary over HTTP: %q", binary)
	}

	if c.expectAsFile && options.AsFile == "" {
		c.t.Errorf("Expecting file, got archive")
	} else if !c.expectAsFile && options.AsFile != "" {
		c.t.Errorf("Expecting archive, got file")
	}

	return &buildapi.Build{}, nil
}

func TestHttpBinary(t *testing.T) {
	tests := []struct {
		description        string
		fromFile           bool // true = --from-file, false = --from-dir/--from-archive
		urlPath            string
		statusCode         int  // server status code, 200 if not set
		contentDisposition bool // will server send Content-Disposition with filename?
		networkError       bool
		tlsBadCert         bool
		expectedError      string
		expectWarning      bool
	}{
		{
			description:        "--from-file, filename in header",
			fromFile:           true,
			urlPath:            "/",
			contentDisposition: true,
		},
		{
			description: "--from-file, filename in URL",
			fromFile:    true,
			urlPath:     "/hi.txt",
		},
		{
			description:   "--from-file, no filename",
			fromFile:      true,
			urlPath:       "",
			expectedError: "unable to determine filename",
		},
		{
			description:   "--from-file, http error",
			fromFile:      true,
			urlPath:       "/",
			statusCode:    404,
			expectedError: "unable to download file",
		},
		{
			description:   "--from-file, network error",
			fromFile:      true,
			urlPath:       "/hi.txt",
			networkError:  true,
			expectedError: "invalid port",
		},
		{
			description:   "--from-file, https with invalid certificate",
			fromFile:      true,
			urlPath:       "/hi.txt",
			tlsBadCert:    true,
			expectedError: "certificate signed by unknown authority",
		},
		{
			description:        "--from-dir, filename in header",
			fromFile:           false,
			contentDisposition: true,
			expectWarning:      true,
		},
		{
			description:   "--from-dir, filename in URL",
			fromFile:      false,
			urlPath:       "/hi.tar.gz",
			expectWarning: true,
		},
		{
			description:   "--from-dir, no filename",
			fromFile:      false,
			expectWarning: true,
		},
		{
			description:   "--from-dir, http error",
			statusCode:    503,
			fromFile:      false,
			expectedError: "unable to download file",
		},
	}

	for _, tc := range tests {
		stdin := bytes.NewReader([]byte{})
		stdout := &bytes.Buffer{}
		options := buildapi.BinaryBuildRequestOptions{}
		handler := func(w http.ResponseWriter, r *http.Request) {
			if tc.contentDisposition {
				w.Header().Add("Content-Disposition", "attachment; filename=hi.txt")
			}
			if tc.statusCode > 0 {
				w.WriteHeader(tc.statusCode)
			}
			w.Write([]byte("hi"))
		}
		var server *httptest.Server
		if tc.tlsBadCert {
			// uses self-signed certificate
			server = httptest.NewTLSServer(http.HandlerFunc(handler))
		} else {
			server = httptest.NewServer(http.HandlerFunc(handler))
		}
		defer server.Close()

		if tc.networkError {
			server.URL = "http://localhost:999999"
		}

		var fromDir, fromFile string
		if tc.fromFile {
			fromFile = server.URL + tc.urlPath
		} else {
			fromDir = server.URL + tc.urlPath
		}

		build, err := streamPathToBuild(nil, stdin, stdout, &FakeBuildConfigs{t: t, expectAsFile: tc.fromFile}, fromDir, fromFile, "", &options)

		if len(tc.expectedError) > 0 {
			if err == nil {
				t.Errorf("[%s] Expected error: %q, got success", tc.description, tc.expectedError)
			} else if !strings.Contains(err.Error(), tc.expectedError) {
				t.Errorf("[%s] Expected error: %q, got: %v", tc.description, tc.expectedError, err)
			}
		} else {
			if err != nil {
				t.Errorf("[%s] Unexpected error: %v", tc.description, err)
				continue
			}

			if build == nil {
				t.Errorf("[%s] No error and no build?", tc.description)
			}

			if tc.fromFile && options.AsFile != "hi.txt" {
				t.Errorf("[%s] Wrong asFile: %q", tc.description, options.AsFile)
			} else if !tc.fromFile && options.AsFile != "" {
				t.Errorf("[%s] asFile set when using --from-dir: %q", tc.description, options.AsFile)
			}
		}

		if out := stdout.String(); tc.expectWarning != strings.Contains(out, "may not be an archive") {
			t.Errorf("[%s] Expected archive warning: %v, got: %q", tc.description, tc.expectWarning, out)
		}
	}
}
