package buildconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kapi "k8s.io/kubernetes/pkg/api"

	_ "github.com/openshift/origin/pkg/api/install"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildapiv1 "github.com/openshift/origin/pkg/build/apis/build/v1"
	"github.com/openshift/origin/pkg/build/registry/test"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/bitbucket"
	"github.com/openshift/origin/pkg/build/webhook/github"
	"github.com/openshift/origin/pkg/build/webhook/gitlab"
	"github.com/openshift/origin/pkg/util/rest"
)

type buildConfigInstantiator struct {
	Build   *buildapi.Build
	Err     error
	Request *buildapi.BuildRequest
}

func (i *buildConfigInstantiator) Instantiate(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	i.Request = request
	if i.Build != nil {
		return i.Build, i.Err
	}
	return &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.Name,
			Namespace: namespace,
		},
	}, i.Err
}

type plugin struct {
	Secret, Path          string
	Err                   error
	Env                   []kapi.EnvVar
	DockerStrategyOptions *buildapi.DockerStrategyOptions
	Proceed               bool
}

func (p *plugin) Extract(buildCfg *buildapi.BuildConfig, secret, path string, req *http.Request) (*buildapi.SourceRevision, []kapi.EnvVar, *buildapi.DockerStrategyOptions, bool, error) {
	p.Secret, p.Path = secret, path
	return nil, p.Env, p.DockerStrategyOptions, p.Proceed, p.Err
}

func newStorage() (*rest.WebHook, *buildConfigInstantiator, *test.BuildConfigRegistry) {
	mockRegistry := &test.BuildConfigRegistry{}
	bci := &buildConfigInstantiator{}
	hook := NewWebHookREST(mockRegistry, bci, buildapiv1.SchemeGroupVersion, map[string]webhook.Plugin{
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
	if out, ok := hook.New().(*buildapi.Build); !ok {
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
		Obj         *buildapi.BuildConfig
		RegErr      error
		ErrFn       func(error) bool
		WFn         func(*httptest.ResponseRecorder) bool
		EnvLen      int
		Instantiate bool
	}{
		"hook returns generic error": {
			Name: "test",
			Path: "secret/err",
			Obj:  &buildapi.BuildConfig{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
			ErrFn: func(err error) bool {
				return strings.Contains(err.Error(), "Internal error occurred: hook failed: test error")
			},
			Instantiate: false,
		},
		"hook returns unauthorized for bad secret": {
			Name:        "test",
			Path:        "secret/errsecret",
			Obj:         &buildapi.BuildConfig{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
			ErrFn:       kerrors.IsUnauthorized,
			Instantiate: false,
		},
		"hook returns unauthorized for bad hook": {
			Name:        "test",
			Path:        "secret/errhook",
			Obj:         &buildapi.BuildConfig{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
			ErrFn:       kerrors.IsUnauthorized,
			Instantiate: false,
		},
		"hook returns unauthorized for missing build config": {
			Name:        "test",
			Path:        "secret/errhook",
			Obj:         &buildapi.BuildConfig{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
			RegErr:      fmt.Errorf("any old error"),
			ErrFn:       kerrors.IsUnauthorized,
			Instantiate: false,
		},
		"hook returns 200 for ok hook": {
			Name:  "test",
			Path:  "secret/ok",
			Obj:   &buildapi.BuildConfig{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
			ErrFn: func(err error) bool { return err == nil },
			WFn: func(w *httptest.ResponseRecorder) bool {
				body, _ := ioutil.ReadAll(w.Body)
				// We want to make sure that we return the created build in the body.
				if w.Code == http.StatusOK && len(body) > 0 {
					// The returned json needs to be a v1 Build specifically
					newBuild := &buildapiv1.Build{}
					err := json.Unmarshal(body, newBuild)
					if err == nil {
						return true
					}
					return false
				}
				return false
			},
			Instantiate: true,
		},
		"hook returns 200 for okenv hook": {
			Name:  "test",
			Path:  "secret/okenv",
			Obj:   &buildapi.BuildConfig{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
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
		handler, err := hook.Connect(apirequest.NewDefaultContext(), testCase.Name, &kapi.PodProxyOptions{Path: testCase.Path}, responder)
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

func (*okBuildConfigInstantiator) Instantiate(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	return &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      request.Name,
		},
	}, nil
}

type errorBuildConfigInstantiator struct{}

func (*errorBuildConfigInstantiator) Instantiate(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	return nil, errors.New("Build error!")
}

type errorBuildConfigGetter struct{}

func (*errorBuildConfigGetter) Get(namespace, name string) (*buildapi.BuildConfig, error) {
	return &buildapi.BuildConfig{}, errors.New("BuildConfig error!")
}

type errorBuildConfigUpdater struct{}

func (*errorBuildConfigUpdater) Update(buildConfig *buildapi.BuildConfig) error {
	return errors.New("BuildConfig error!")
}

type pathPlugin struct {
	Path string
}

func (p *pathPlugin) Extract(buildCfg *buildapi.BuildConfig, secret, path string, req *http.Request) (*buildapi.SourceRevision, []kapi.EnvVar, *buildapi.DockerStrategyOptions, bool, error) {
	p.Path = path
	return nil, []kapi.EnvVar{}, nil, true, nil
}

type errPlugin struct{}

func (*errPlugin) Extract(buildCfg *buildapi.BuildConfig, secret, path string, req *http.Request) (*buildapi.SourceRevision, []kapi.EnvVar, *buildapi.DockerStrategyOptions, bool, error) {
	return nil, []kapi.EnvVar{}, nil, false, errors.New("Plugin error!")
}

var testBuildConfig = &buildapi.BuildConfig{
	ObjectMeta: metav1.ObjectMeta{Name: "build100"},
	Spec: buildapi.BuildConfigSpec{
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				Type: buildapi.GitHubWebHookBuildTriggerType,
				GitHubWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
			{
				Type: buildapi.GitLabWebHookBuildTriggerType,
				GitLabWebHook: &buildapi.WebHookTrigger{
					Secret: "secret201",
				},
			},
			{
				Type: buildapi.BitbucketWebHookBuildTriggerType,
				BitbucketWebHook: &buildapi.WebHookTrigger{
					Secret: "secret301",
				},
			},
		},
		CommonSpec: buildapi.CommonSpec{
			Source: buildapi.BuildSource{
				Git: &buildapi.GitBuildSource{
					URI: "git://github.com/my/repo.git",
				},
			},
			Strategy: mockBuildStrategy,
		},
	},
}
var mockBuildStrategy = buildapi.BuildStrategy{
	SourceStrategy: &buildapi.SourceBuildStrategy{
		From: kapi.ObjectReference{
			Kind: "DockerImage",
			Name: "repository/image",
		},
	},
}

func TestParseUrlError(t *testing.T) {
	bcRegistry := &test.BuildConfigRegistry{BuildConfig: testBuildConfig}
	responder := &fakeResponder{}
	handler, _ := NewWebHookREST(bcRegistry, &okBuildConfigInstantiator{}, buildapiv1.SchemeGroupVersion, map[string]webhook.Plugin{"github": github.New(), "gitlab": gitlab.New(), "bitbucket": bitbucket.New()}).
		Connect(apirequest.NewDefaultContext(), "build100", &kapi.PodProxyOptions{Path: ""}, responder)
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
	handler, _ := NewWebHookREST(bcRegistry, &okBuildConfigInstantiator{}, buildapiv1.SchemeGroupVersion, map[string]webhook.Plugin{"pathplugin": &pathPlugin{}}).
		Connect(apirequest.NewDefaultContext(), "build100", &kapi.PodProxyOptions{Path: "secret101/pathplugin"}, responder)
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
	handler, _ := NewWebHookREST(bcRegistry, &okBuildConfigInstantiator{}, buildapiv1.SchemeGroupVersion, map[string]webhook.Plugin{"pathplugin": plugin}).
		Connect(apirequest.NewDefaultContext(), "build100", &kapi.PodProxyOptions{Path: "secret101/pathplugin/some/more/args"}, responder)
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
	handler, _ := NewWebHookREST(bcRegistry, &okBuildConfigInstantiator{}, buildapiv1.SchemeGroupVersion, map[string]webhook.Plugin{"pathplugin": &pathPlugin{}}).
		Connect(apirequest.NewDefaultContext(), "build100", &kapi.PodProxyOptions{Path: "secret101/missingplugin"}, responder)
	server := httptest.NewServer(handler)
	defer server.Close()

	_, err := http.Post(server.URL, "application/json", nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !responder.called ||
		!strings.Contains(responder.err.Error(), `buildconfighook "missingplugin" not found`) {
		t.Errorf("Expected BadRequest, got %s, expected error %s!", responder.err.Error(), `buildconfighook.build.openshift.io "missingplugin" not found`)
	}
}

func TestInvokeWebhookErrorBuildConfigInstantiate(t *testing.T) {
	bcRegistry := &test.BuildConfigRegistry{BuildConfig: testBuildConfig}
	responder := &fakeResponder{}
	handler, _ := NewWebHookREST(bcRegistry, &errorBuildConfigInstantiator{}, buildapiv1.SchemeGroupVersion, map[string]webhook.Plugin{"pathplugin": &pathPlugin{}}).
		Connect(apirequest.NewDefaultContext(), "build100", &kapi.PodProxyOptions{Path: "secret101/pathplugin"}, responder)
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
	handler, _ := NewWebHookREST(bcRegistry, &okBuildConfigInstantiator{}, buildapiv1.SchemeGroupVersion, map[string]webhook.Plugin{"pathplugin": &pathPlugin{}}).
		Connect(apirequest.NewDefaultContext(), "badbuild100", &kapi.PodProxyOptions{Path: "secret101/pathplugin"}, responder)
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
	handler, _ := NewWebHookREST(bcRegistry, &okBuildConfigInstantiator{}, buildapiv1.SchemeGroupVersion, map[string]webhook.Plugin{"errPlugin": &errPlugin{}}).
		Connect(apirequest.NewDefaultContext(), "build100", &kapi.PodProxyOptions{Path: "secret101/errPlugin"}, responder)
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
	revision := &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
			Author: buildapi.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Committer: buildapi.SourceControlUser{
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
		if cause.Message != buildapi.BuildTriggerCauseGenericMsg {
			t.Errorf("Expected build reason to be 'Generic WebHook, go %s'", cause.Message)
		}
	}
}

func TestGeneratedBuildTriggerInfoGitHubWebHook(t *testing.T) {
	revision := &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
			Author: buildapi.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Committer: buildapi.SourceControlUser{
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
		if cause.Message != buildapi.BuildTriggerCauseGithubMsg {
			t.Errorf("Expected build reason to be 'GitHub WebHook, go %s'", cause.Message)
		}
	}
}

func TestGeneratedBuildTriggerInfoGitLabWebHook(t *testing.T) {
	revision := &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
			Author: buildapi.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Committer: buildapi.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Message: "A random act of kindness",
		},
	}

	buildtriggerCause := generateBuildTriggerInfo(revision, "gitlab", "mysecret")
	hiddenSecret := fmt.Sprintf("%s***", "mysecret"[:(len("mysecret")/2)])
	for _, cause := range buildtriggerCause {
		if !reflect.DeepEqual(revision, cause.GitLabWebHook.Revision) {
			t.Errorf("Expected returned revision to equal: %v", revision)
		}
		if cause.GitLabWebHook.Secret != hiddenSecret {
			t.Errorf("Expected obfuscated secret to be: %s", hiddenSecret)
		}
		if cause.Message != buildapi.BuildTriggerCauseGitLabMsg {
			t.Errorf("Expected build reason to be 'GitLab WebHook, go %s'", cause.Message)
		}
	}
}

func TestGeneratedBuildTriggerInfoBitbucketWebHook(t *testing.T) {
	revision := &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
			Author: buildapi.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Committer: buildapi.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Message: "A random act of kindness",
		},
	}

	buildtriggerCause := generateBuildTriggerInfo(revision, "bitbucket", "mysecret")
	hiddenSecret := fmt.Sprintf("%s***", "mysecret"[:(len("mysecret")/2)])
	for _, cause := range buildtriggerCause {
		if !reflect.DeepEqual(revision, cause.BitbucketWebHook.Revision) {
			t.Errorf("Expected returned revision to equal: %v", revision)
		}
		if cause.BitbucketWebHook.Secret != hiddenSecret {
			t.Errorf("Expected obfuscated secret to be: %s", hiddenSecret)
		}
		if cause.Message != buildapi.BuildTriggerCauseBitbucketMsg {
			t.Errorf("Expected build reason to be 'Bitbucket WebHook, go %s'", cause.Message)
		}
	}
}
