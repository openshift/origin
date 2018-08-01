package startbuild

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

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	restclient "k8s.io/client-go/rest"
	restfake "k8s.io/client-go/rest/fake"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	buildv1 "github.com/openshift/api/build/v1"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclientmanual "github.com/openshift/origin/pkg/build/client/v1"
	"github.com/openshift/origin/pkg/oauth/generated/internalclientset/scheme"
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
	o := &StartBuildOptions{
		IOStreams:    genericclioptions.NewTestIOStreamsDiscard(),
		ClientConfig: cfg.Client,
		FromWebhook:  server.URL + "/webhook",
		Mapper:       testrestmapper.TestOnlyStaticRESTMapper(legacyscheme.Scheme),
	}
	if err := o.Run(); err != nil {
		t.Fatalf("unable to start hook: %v", err)
	}
	<-invoked

	o = &StartBuildOptions{
		IOStreams:      genericclioptions.NewTestIOStreamsDiscard(),
		FromWebhook:    server.URL + "/webhook",
		GitPostReceive: "unknownpath",
	}
	if err := o.Run(); err == nil {
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
	o := &StartBuildOptions{
		IOStreams:      genericclioptions.NewTestIOStreamsDiscard(),
		ClientConfig:   cfg.Client,
		FromWebhook:    server.URL + "/webhook",
		GitPostReceive: f.Name(),
		Mapper:         testrestmapper.TestOnlyStaticRESTMapper(legacyscheme.Scheme),
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

func (c FakeBuildConfigs) InstantiateBinary(name string, options *buildv1.BinaryBuildRequestOptions, r io.Reader) (result *buildv1.Build, err error) {
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

	return &buildv1.Build{}, nil
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
		options := buildv1.BinaryBuildRequestOptions{}
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

type logTestCase struct {
	RequestErr     error
	IOErr          error
	ExpectedLogMsg string
	ExpectedErrMsg string
}

type failReader struct {
	Err error
}

func (r *failReader) Read(p []byte) (n int, err error) {
	return 0, r.Err
}

func TestStreamBuildLogs(t *testing.T) {
	cases := []logTestCase{
		{
			ExpectedLogMsg: "hello",
		},
		{
			RequestErr:     errors.New("simulated failure"),
			ExpectedErrMsg: "unable to stream the build logs",
		},
		{
			RequestErr: &kerrors.StatusError{
				ErrStatus: metav1.Status{
					Reason:  metav1.StatusReasonTimeout,
					Message: "timeout",
				},
			},
			ExpectedErrMsg: "unable to stream the build logs",
		},
		{
			IOErr:          errors.New("failed to read"),
			ExpectedErrMsg: "unable to stream the build logs",
		},
	}

	for _, tc := range cases {
		out := &bytes.Buffer{}
		build := &buildv1.Build{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-build",
				Namespace: "test-namespace",
			},
		}
		// Set up dummy RESTClient to handle requests
		fakeREST := &restfake.RESTClient{
			NegotiatedSerializer: scheme.Codecs,
			GroupVersion:         schema.GroupVersion{Group: "build.openshift.io", Version: "v1"},
			Client: restfake.CreateHTTPClient(func(*http.Request) (*http.Response, error) {
				if tc.RequestErr != nil {
					return nil, tc.RequestErr
				}
				var body io.Reader
				if tc.IOErr != nil {
					body = &failReader{
						Err: tc.IOErr,
					}
				} else {
					body = bytes.NewBufferString(tc.ExpectedLogMsg)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(body),
				}, nil
			}),
		}

		ioStreams, _, out, _ := genericclioptions.NewTestIOStreams()

		o := &StartBuildOptions{
			IOStreams:      ioStreams,
			BuildLogClient: buildclientmanual.NewBuildLogClient(fakeREST, build.Namespace),
		}

		err := o.streamBuildLogs(build)
		if tc.RequestErr == nil && tc.IOErr == nil {
			if err != nil {
				t.Errorf("received unexpected error streaming build logs: %v", err)
			}
			if out.String() != tc.ExpectedLogMsg {
				t.Errorf("expected log \"%s\", got \"%s\"", tc.ExpectedLogMsg, out.String())
			}
		} else {
			if err == nil {
				t.Errorf("no error was received, expected error message: %s", tc.ExpectedErrMsg)
			} else if !strings.Contains(err.Error(), tc.ExpectedErrMsg) {
				t.Errorf("expected error message \"%s\", got \"%s\"", tc.ExpectedErrMsg, err)
			}
		}
	}
}
