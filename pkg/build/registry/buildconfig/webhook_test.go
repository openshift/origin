package buildconfig

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/test"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/github"
	"github.com/openshift/origin/pkg/util/rest"
)

type buildConfigInstantiator struct {
	Build   *api.Build
	Err     error
	Request *api.BuildRequest
}

func (i *buildConfigInstantiator) Instantiate(namespace string, request *api.BuildRequest) (*api.Build, error) {
	i.Request = request
	return i.Build, i.Err
}

type plugin struct {
	Secret, Path string
	Err          error
	Env          []kapi.EnvVar
	Proceed      bool
}

func (p *plugin) Extract(buildCfg *api.BuildConfig, secret, path string, req *http.Request) (*api.SourceRevision, []kapi.EnvVar, bool, error) {
	p.Secret, p.Path = secret, path
	return nil, p.Env, p.Proceed, p.Err
}

func newStorage() (*rest.WebHook, *buildConfigInstantiator, *test.BuildConfigRegistry) {
	mockRegistry := &test.BuildConfigRegistry{}
	bci := &buildConfigInstantiator{}
	hook := NewWebHookREST(mockRegistry, bci, map[string]webhook.Plugin{
		"ok": &plugin{Proceed: true},
		"okenv": &plugin{
			Env: []kapi.EnvVar{
				{
					Name:  "foo",
					Value: "bar",
				},
			},
			Proceed: true,
		},
		"errsecret": &plugin{Err: webhook.ErrSecretMismatch},
		"errhook":   &plugin{Err: webhook.ErrHookNotEnabled},
		"err":       &plugin{Err: fmt.Errorf("test error")},
	})
	return hook, bci, mockRegistry
}

func TestNewWebHook(t *testing.T) {
	hook, _, _ := newStorage()
	if out, ok := hook.New().(*unversioned.Status); !ok {
		t.Errorf("unexpected new: %#v", out)
	}
}

type fakeResponder struct {
	called     bool
	statusCode int
	object     runtime.Object
	err        error
}

func (r *fakeResponder) Object(statusCode int, obj runtime.Object) {
	if r.called {
		panic("called twice")
	}
	r.called = true
	r.statusCode = statusCode
	r.object = obj
}

func (r *fakeResponder) Error(err error) {
	if r.called {
		panic("called twice")
	}
	r.called = true
	r.err = err
}

func TestConnectWebHook(t *testing.T) {
	testCases := map[string]struct {
		Name        string
		Path        string
		Obj         *api.BuildConfig
		RegErr      error
		ErrFn       func(error) bool
		WFn         func(*httptest.ResponseRecorder) bool
		EnvLen      int
		Instantiate bool
	}{
		"hook returns generic error": {
			Name: "test",
			Path: "secret/err",
			Obj:  &api.BuildConfig{ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "default"}},
			ErrFn: func(err error) bool {
				return strings.Contains(err.Error(), "Internal error occurred: hook failed: test error")
			},
			Instantiate: false,
		},
		"hook returns unauthorized for bad secret": {
			Name:        "test",
			Path:        "secret/errsecret",
			Obj:         &api.BuildConfig{ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "default"}},
			ErrFn:       kerrors.IsUnauthorized,
			Instantiate: false,
		},
		"hook returns unauthorized for bad hook": {
			Name:        "test",
			Path:        "secret/errhook",
			Obj:         &api.BuildConfig{ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "default"}},
			ErrFn:       kerrors.IsUnauthorized,
			Instantiate: false,
		},
		"hook returns unauthorized for missing build config": {
			Name:        "test",
			Path:        "secret/errhook",
			Obj:         &api.BuildConfig{ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "default"}},
			RegErr:      fmt.Errorf("any old error"),
			ErrFn:       kerrors.IsUnauthorized,
			Instantiate: false,
		},
		"hook returns 200 for ok hook": {
			Name:  "test",
			Path:  "secret/ok",
			Obj:   &api.BuildConfig{ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "default"}},
			ErrFn: func(err error) bool { return err == nil },
			WFn: func(w *httptest.ResponseRecorder) bool {
				return w.Code == http.StatusOK
			},
			Instantiate: true,
		},
		"hook returns 200 for okenv hook": {
			Name:  "test",
			Path:  "secret/okenv",
			Obj:   &api.BuildConfig{ObjectMeta: kapi.ObjectMeta{Name: "test", Namespace: "default"}},
			ErrFn: func(err error) bool { return err == nil },
			WFn: func(w *httptest.ResponseRecorder) bool {
				return w.Code == http.StatusOK
			},
			EnvLen:      1,
			Instantiate: true,
		},
	}
	for k, testCase := range testCases {
		hook, bci, registry := newStorage()
		if testCase.Obj != nil {
			registry.BuildConfig = testCase.Obj
		}
		if testCase.RegErr != nil {
			registry.Err = testCase.RegErr
		}
		responder := &fakeResponder{}
		handler, err := hook.Connect(kapi.NewDefaultContext(), testCase.Name, &kapi.PodProxyOptions{Path: testCase.Path}, responder)
		if err != nil {
			t.Errorf("%s: %v", k, err)
			continue
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, &http.Request{})
		if err := responder.err; !testCase.ErrFn(err) {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		if testCase.WFn != nil && !testCase.WFn(w) {
			t.Errorf("%s: unexpected response: %#v", k, w)
			continue
		}
		if testCase.Instantiate {
			if bci.Request == nil {
				t.Errorf("%s: instantiator not invoked", k)
				continue
			}
			if bci.Request.Name != testCase.Obj.Name {
				t.Errorf("%s: instantiator incorrect: %#v", k, bci)
				continue
			}
		} else {
			if bci.Request != nil {
				t.Errorf("%s: instantiator should not be invoked: %#v", k, bci)
				continue
			}
		}
		if bci.Request != nil && testCase.EnvLen != len(bci.Request.Env) {
			t.Errorf("%s: build request does not have correct env vars:  %+v \n", k, bci.Request)
		}
	}
}

type okBuildConfigInstantiator struct{}

func (*okBuildConfigInstantiator) Instantiate(namespace string, request *api.BuildRequest) (*api.Build, error) {
	return &api.Build{}, nil
}

type errorBuildConfigInstantiator struct{}

func (*errorBuildConfigInstantiator) Instantiate(namespace string, request *api.BuildRequest) (*api.Build, error) {
	return nil, errors.New("Build error!")
}

type errorBuildConfigGetter struct{}

func (*errorBuildConfigGetter) Get(namespace, name string) (*api.BuildConfig, error) {
	return &api.BuildConfig{}, errors.New("BuildConfig error!")
}

type errorBuildConfigUpdater struct{}

func (*errorBuildConfigUpdater) Update(buildConfig *api.BuildConfig) error {
	return errors.New("BuildConfig error!")
}

type pathPlugin struct {
	Path string
}

func (p *pathPlugin) Extract(buildCfg *api.BuildConfig, secret, path string, req *http.Request) (*api.SourceRevision, []kapi.EnvVar, bool, error) {
	p.Path = path
	return nil, []kapi.EnvVar{}, true, nil
}

type errPlugin struct{}

func (*errPlugin) Extract(buildCfg *api.BuildConfig, secret, path string, req *http.Request) (*api.SourceRevision, []kapi.EnvVar, bool, error) {
	return nil, []kapi.EnvVar{}, false, errors.New("Plugin error!")
}

var testBuildConfig = &api.BuildConfig{
	ObjectMeta: kapi.ObjectMeta{Name: "build100"},
	Spec: api.BuildConfigSpec{
		Triggers: []api.BuildTriggerPolicy{
			{
				Type: api.GitHubWebHookBuildTriggerType,
				GitHubWebHook: &api.WebHookTrigger{
					Secret: "secret101",
				},
			},
		},
		CommonSpec: api.CommonSpec{
			Source: api.BuildSource{
				Git: &api.GitBuildSource{
					URI: "git://github.com/my/repo.git",
				},
			},
			Strategy: mockBuildStrategy,
		},
	},
}
var mockBuildStrategy = api.BuildStrategy{
	SourceStrategy: &api.SourceBuildStrategy{
		From: kapi.ObjectReference{
			Kind: "DockerImage",
			Name: "repository/image",
		},
	},
}

func TestParseUrlError(t *testing.T) {
	bcRegistry := &test.BuildConfigRegistry{BuildConfig: testBuildConfig}
	responder := &fakeResponder{}
	handler, _ := NewWebHookREST(bcRegistry, &okBuildConfigInstantiator{}, map[string]webhook.Plugin{"github": github.New()}).
		Connect(kapi.NewDefaultContext(), "build100", &kapi.PodProxyOptions{Path: ""}, responder)
	server := httptest.NewServer(handler)
	defer server.Close()

	_, err := http.Post(server.URL, "application/json", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !responder.called ||
		!strings.Contains(responder.err.Error(), "unexpected hook subpath") {
		t.Errorf("Expected BadRequest, got %s, expected error %s!", responder.err.Error(), "unexpected hook subpath")
	}
}

func TestParseUrlOK(t *testing.T) {
	bcRegistry := &test.BuildConfigRegistry{BuildConfig: testBuildConfig}
	responder := &fakeResponder{}
	handler, _ := NewWebHookREST(bcRegistry, &okBuildConfigInstantiator{}, map[string]webhook.Plugin{"pathplugin": &pathPlugin{}}).
		Connect(kapi.NewDefaultContext(), "build100", &kapi.PodProxyOptions{Path: "secret101/pathplugin"}, responder)
	server := httptest.NewServer(handler)
	defer server.Close()

	_, err := http.Post(server.URL, "application/json", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if responder.err != nil {
		t.Errorf("Expected no error, got %v", responder.err)
	}
}

func TestParseUrlLong(t *testing.T) {
	plugin := &pathPlugin{}
	bcRegistry := &test.BuildConfigRegistry{BuildConfig: testBuildConfig}
	responder := &fakeResponder{}
	handler, _ := NewWebHookREST(bcRegistry, &okBuildConfigInstantiator{}, map[string]webhook.Plugin{"pathplugin": plugin}).
		Connect(kapi.NewDefaultContext(), "build100", &kapi.PodProxyOptions{Path: "secret101/pathplugin/some/more/args"}, responder)
	server := httptest.NewServer(handler)
	defer server.Close()

	_, err := http.Post(server.URL, "application/json", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !responder.called ||
		!strings.Contains(responder.err.Error(), "unexpected hook subpath") {
		t.Errorf("Expected BadRequest, got %s, expected error %s!", responder.err.Error(), "unexpected hook subpath")
	}
}

func TestInvokeWebhookMissingPlugin(t *testing.T) {
	bcRegistry := &test.BuildConfigRegistry{BuildConfig: testBuildConfig}
	responder := &fakeResponder{}
	handler, _ := NewWebHookREST(bcRegistry, &okBuildConfigInstantiator{}, map[string]webhook.Plugin{"pathplugin": &pathPlugin{}}).
		Connect(kapi.NewDefaultContext(), "build100", &kapi.PodProxyOptions{Path: "secret101/missingplugin"}, responder)
	server := httptest.NewServer(handler)
	defer server.Close()

	_, err := http.Post(server.URL, "application/json", nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !responder.called ||
		!strings.Contains(responder.err.Error(), "buildconfighook \"missingplugin\" not found") {
		t.Errorf("Expected BadRequest, got %s, expected error %s!", responder.err.Error(), "buildconfighook \"missingplugin\" not found")
	}
}

func TestInvokeWebhookErrorBuildConfigInstantiate(t *testing.T) {
	bcRegistry := &test.BuildConfigRegistry{BuildConfig: testBuildConfig}
	responder := &fakeResponder{}
	handler, _ := NewWebHookREST(bcRegistry, &errorBuildConfigInstantiator{}, map[string]webhook.Plugin{"pathplugin": &pathPlugin{}}).
		Connect(kapi.NewDefaultContext(), "build100", &kapi.PodProxyOptions{Path: "secret101/pathplugin"}, responder)
	server := httptest.NewServer(handler)
	defer server.Close()

	_, err := http.Post(server.URL, "application/json", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !responder.called ||
		!strings.Contains(responder.err.Error(), "could not generate a build") {
		t.Errorf("Expected BadRequest, got %s, expected error %s!", responder.err.Error(), "could not generate a build")
	}
}

func TestInvokeWebhookErrorGetConfig(t *testing.T) {
	bcRegistry := &test.BuildConfigRegistry{BuildConfig: testBuildConfig}
	responder := &fakeResponder{}
	handler, _ := NewWebHookREST(bcRegistry, &okBuildConfigInstantiator{}, map[string]webhook.Plugin{"pathplugin": &pathPlugin{}}).
		Connect(kapi.NewDefaultContext(), "badbuild100", &kapi.PodProxyOptions{Path: "secret101/pathplugin"}, responder)
	server := httptest.NewServer(handler)
	defer server.Close()

	_, err := http.Post(server.URL, "application/json", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !responder.called {
		t.Fatalf("Should have received an error")
	}
	if !strings.Contains(responder.err.Error(), "did not accept your secret") {
		t.Errorf("Expected BadRequest, got %s, expected error %s!", responder.err.Error(), "did not accept your secret")
	}
}

func TestInvokeWebhookErrorCreateBuild(t *testing.T) {
	bcRegistry := &test.BuildConfigRegistry{BuildConfig: testBuildConfig}
	responder := &fakeResponder{}
	handler, _ := NewWebHookREST(bcRegistry, &okBuildConfigInstantiator{}, map[string]webhook.Plugin{"errPlugin": &errPlugin{}}).
		Connect(kapi.NewDefaultContext(), "build100", &kapi.PodProxyOptions{Path: "secret101/errPlugin"}, responder)
	server := httptest.NewServer(handler)
	defer server.Close()

	_, err := http.Post(server.URL, "application/json", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !responder.called ||
		!strings.Contains(responder.err.Error(), "Internal error occurred: hook failed: Plugin error!") {
		t.Errorf("Expected BadRequest, got %s, expected error %s!", responder.err.Error(), "Internal error occurred: hook failed: Plugin error!")
	}
}

func TestGeneratedBuildTriggerInfoGenericWebHook(t *testing.T) {
	revision := &api.SourceRevision{
		Git: &api.GitSourceRevision{
			Author: api.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Committer: api.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Message: "A random act of kindness",
		},
	}

	buildtriggerCause := generateBuildTriggerInfo(revision, "generic", "mysecret")
	hiddenSecret := fmt.Sprintf("%s***", "mysecret"[:(len("mysecret")/2)])
	for _, cause := range buildtriggerCause {
		if !reflect.DeepEqual(revision, cause.GenericWebHook.Revision) {
			t.Errorf("Expected returned revision to equal: %v", revision)
		}
		if cause.GenericWebHook.Secret != hiddenSecret {
			t.Errorf("Expected obfuscated secret to be: %s", hiddenSecret)
		}
		if cause.Message != api.BuildTriggerCauseGenericMsg {
			t.Errorf("Expected build reason to be 'Generic WebHook, go %s'", cause.Message)
		}
	}
}

func TestGeneratedBuildTriggerInfoGitHubWebHook(t *testing.T) {
	revision := &api.SourceRevision{
		Git: &api.GitSourceRevision{
			Author: api.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Committer: api.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Message: "A random act of kindness",
		},
	}

	buildtriggerCause := generateBuildTriggerInfo(revision, "github", "mysecret")
	hiddenSecret := fmt.Sprintf("%s***", "mysecret"[:(len("mysecret")/2)])
	for _, cause := range buildtriggerCause {
		if !reflect.DeepEqual(revision, cause.GitHubWebHook.Revision) {
			t.Errorf("Expected returned revision to equal: %v", revision)
		}
		if cause.GitHubWebHook.Secret != hiddenSecret {
			t.Errorf("Expected obfuscated secret to be: %s", hiddenSecret)
		}
		if cause.Message != api.BuildTriggerCauseGithubMsg {
			t.Errorf("Expected build reason to be 'GitHub WebHook, go %s'", cause.Message)
		}
	}
}
