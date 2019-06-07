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

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	clientesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildv1 "github.com/openshift/api/build/v1"
	buildfake "github.com/openshift/client-go/build/clientset/versioned/fake"
	buildclientv1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"

	"github.com/openshift/origin/pkg/build/apiserver/apiserverbuildutil"
	"github.com/openshift/origin/pkg/build/apiserver/webhook"
	"github.com/openshift/origin/pkg/build/apiserver/webhook/bitbucket"
	"github.com/openshift/origin/pkg/build/apiserver/webhook/github"
	"github.com/openshift/origin/pkg/build/apiserver/webhook/gitlab"
	// _ "github.com/openshift/origin/pkg/api/install"
)

type fakeInstantiator interface {
	Instantiate(buildConfigName string, buildRequest *buildv1.BuildRequest) (*buildv1.Build, error)
}

type fakeBuildConfigInterface struct {
	inst      fakeInstantiator
	client    buildclientv1.BuildConfigInterface
	namespace string
}

func (f *fakeBuildConfigInterface) Create(build *buildv1.BuildConfig) (*buildv1.BuildConfig, error) {
	return f.client.Create(build)
}

func (f *fakeBuildConfigInterface) Update(build *buildv1.BuildConfig) (*buildv1.BuildConfig, error) {
	return f.client.Update(build)
}

func (f *fakeBuildConfigInterface) UpdateStatus(build *buildv1.BuildConfig) (*buildv1.BuildConfig, error) {
	return f.client.UpdateStatus(build)
}

func (f *fakeBuildConfigInterface) Delete(name string, options *metav1.DeleteOptions) error {
	return f.client.Delete(name, options)
}

func (f *fakeBuildConfigInterface) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	panic("implement me")
}

func (f *fakeBuildConfigInterface) Get(name string, options metav1.GetOptions) (*buildv1.BuildConfig, error) {
	return f.client.Get(name, options)
}

func (f *fakeBuildConfigInterface) List(opts metav1.ListOptions) (*buildv1.BuildConfigList, error) {
	return f.client.List(opts)
}

func (f *fakeBuildConfigInterface) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return f.client.Watch(opts)
}

func (f *fakeBuildConfigInterface) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *buildv1.BuildConfig, err error) {
	return f.client.Patch(name, pt, data, subresources...)
}

func (f *fakeBuildConfigInterface) Instantiate(buildConfigName string, buildRequest *buildv1.BuildRequest) (*buildv1.Build, error) {
	return f.inst.Instantiate(f.namespace, buildRequest)
}

type fakeBuildConfigClient struct {
	inst       fakeInstantiator
	client     buildclientv1.BuildConfigsGetter
	fakeclient *buildfake.Clientset
}

func (i *fakeBuildConfigClient) BuildConfigs(namespace string) buildclientv1.BuildConfigInterface {
	return &fakeBuildConfigInterface{inst: i.inst, client: i.client.BuildConfigs(namespace)}
}

func newBuildConfigClient(inst fakeInstantiator, objs ...runtime.Object) buildclientv1.BuildConfigsGetter {
	client := buildfake.NewSimpleClientset(objs...)
	return &fakeBuildConfigClient{
		inst:       inst,
		fakeclient: client,
		client:     client.BuildV1(),
	}
}

type buildConfigInstantiator struct {
	Build   *buildv1.Build
	Err     error
	Request *buildv1.BuildRequest
}

func (i *buildConfigInstantiator) Instantiate(namespace string, request *buildv1.BuildRequest) (*buildv1.Build, error) {
	i.Request = request
	if i.Build != nil {
		return i.Build, i.Err
	}
	return &buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.Name,
			Namespace: namespace,
		},
	}, i.Err
}

type plugin struct {
	Triggers              []*buildv1.WebHookTrigger
	Err                   error
	Env                   []corev1.EnvVar
	DockerStrategyOptions *buildv1.DockerStrategyOptions
	Proceed               bool
}

func (p *plugin) Extract(buildCfg *buildv1.BuildConfig, trigger *buildv1.WebHookTrigger, req *http.Request) (*buildv1.SourceRevision, []corev1.EnvVar, *buildv1.DockerStrategyOptions, bool, error) {
	p.Triggers = []*buildv1.WebHookTrigger{trigger}
	return nil, p.Env, p.DockerStrategyOptions, p.Proceed, p.Err
}
func (p *plugin) GetTriggers(buildConfig *buildv1.BuildConfig) ([]*buildv1.WebHookTrigger, error) {
	trigger := &buildv1.WebHookTrigger{
		Secret: "secret",
	}
	return []*buildv1.WebHookTrigger{trigger}, nil
}
func newStorage() (*WebHook, *buildConfigInstantiator, *buildfake.Clientset) {
	bci := &buildConfigInstantiator{}
	fakeBuildClient := newBuildConfigClient(bci)
	plugins := map[string]webhook.Plugin{
		"ok": &plugin{Proceed: true},
		"okenv": &plugin{
			Env: []corev1.EnvVar{
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
	}
	hook := newWebHookREST(fakeBuildClient, nil, buildv1.SchemeGroupVersion, plugins)

	return hook, bci, fakeBuildClient.(*fakeBuildConfigClient).fakeclient
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
		Obj         *buildv1.BuildConfig
		RegErr      error
		ErrFn       func(error) bool
		WFn         func(*httptest.ResponseRecorder) bool
		EnvLen      int
		Instantiate bool
	}{
		"hook returns generic error": {
			Name: "test",
			Path: "secret/err",
			Obj:  &buildv1.BuildConfig{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
			ErrFn: func(err error) bool {
				return strings.Contains(err.Error(), "Internal error occurred: hook failed: test error")
			},
			Instantiate: false,
		},
		"hook returns unauthorized for bad secret": {
			Name:        "test",
			Path:        "secret/errsecret",
			Obj:         &buildv1.BuildConfig{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
			ErrFn:       kerrors.IsUnauthorized,
			Instantiate: false,
		},
		"hook returns unauthorized for bad hook": {
			Name:        "test",
			Path:        "secret/errhook",
			Obj:         &buildv1.BuildConfig{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
			ErrFn:       kerrors.IsUnauthorized,
			Instantiate: false,
		},
		"hook returns unauthorized for missing build config": {
			Name:        "test",
			Path:        "secret/errhook",
			Obj:         &buildv1.BuildConfig{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
			RegErr:      fmt.Errorf("any old error"),
			ErrFn:       kerrors.IsUnauthorized,
			Instantiate: false,
		},
		"hook returns 200 for ok hook": {
			Name:  "test",
			Path:  "secret/ok",
			Obj:   &buildv1.BuildConfig{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
			ErrFn: func(err error) bool { return err == nil },
			WFn: func(w *httptest.ResponseRecorder) bool {
				body, _ := ioutil.ReadAll(w.Body)
				// We want to make sure that we return the created build in the body.
				if w.Code == http.StatusOK && len(body) > 0 {
					// The returned json needs to be a buildclientv1 Build specifically
					newBuild := &buildv1.Build{}
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
			Obj:   &buildv1.BuildConfig{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
			ErrFn: func(err error) bool { return err == nil },
			WFn: func(w *httptest.ResponseRecorder) bool {
				return w.Code == http.StatusOK
			},
			EnvLen:      1,
			Instantiate: true,
		},
	}
	for k, testCase := range testCases {
		hook, bci, fakeBuildClient := newStorage()
		if testCase.Obj != nil {
			fakeBuildClient.PrependReactor("get", "buildconfigs", func(action clientesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, testCase.Obj, nil
			})
		}
		if testCase.RegErr != nil {
			fakeBuildClient.PrependReactor("get", "buildconfigs", func(action clientesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, testCase.RegErr
			})
		}
		responder := &fakeResponder{}
		handler, err := hook.Connect(apirequest.WithNamespace(apirequest.NewDefaultContext(), testBuildConfig.Namespace), testCase.Name, &kapi.PodProxyOptions{Path: testCase.Path}, responder)
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

func (*okBuildConfigInstantiator) Instantiate(namespace string, request *buildv1.BuildRequest) (*buildv1.Build, error) {
	return &buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      request.Name,
		},
	}, nil
}

type errorBuildConfigInstantiator struct{}

func (*errorBuildConfigInstantiator) Instantiate(namespace string, request *buildv1.BuildRequest) (*buildv1.Build, error) {
	return nil, errors.New("Build error!")
}

type errorBuildConfigGetter struct{}

func (*errorBuildConfigGetter) Get(namespace, name string) (*buildv1.BuildConfig, error) {
	return &buildv1.BuildConfig{}, errors.New("BuildConfig error!")
}

type errorBuildConfigUpdater struct{}

func (*errorBuildConfigUpdater) Update(buildConfig *buildv1.BuildConfig) error {
	return errors.New("BuildConfig error!")
}

type pathPlugin struct {
}

func (p *pathPlugin) Extract(buildCfg *buildv1.BuildConfig, trigger *buildv1.WebHookTrigger, req *http.Request) (*buildv1.SourceRevision,
	[]corev1.EnvVar, *buildv1.DockerStrategyOptions, bool, error) {
	return nil, []corev1.EnvVar{}, nil, true, nil
}

func (p *pathPlugin) GetTriggers(buildConfig *buildv1.BuildConfig) ([]*buildv1.WebHookTrigger, error) {
	trigger := &buildv1.WebHookTrigger{
		Secret: "secret101",
	}
	return []*buildv1.WebHookTrigger{trigger}, nil
}

type errPlugin struct {
}

func (*errPlugin) Extract(buildCfg *buildv1.BuildConfig, trigger *buildv1.WebHookTrigger, req *http.Request) (*buildv1.SourceRevision,
	[]corev1.EnvVar, *buildv1.DockerStrategyOptions, bool, error) {
	return nil, []corev1.EnvVar{}, nil, false, errors.New("Plugin error!")
}
func (p *errPlugin) GetTriggers(buildConfig *buildv1.BuildConfig) ([]*buildv1.WebHookTrigger, error) {
	trigger := &buildv1.WebHookTrigger{
		Secret: "secret101",
	}
	return []*buildv1.WebHookTrigger{trigger}, nil
}

var testBuildConfig = &buildv1.BuildConfig{
	ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "build100"},
	Spec: buildv1.BuildConfigSpec{
		Triggers: []buildv1.BuildTriggerPolicy{
			{
				Type: buildv1.GitHubWebHookBuildTriggerType,
				GitHubWebHook: &buildv1.WebHookTrigger{
					Secret: "secret101",
				},
			},
			{
				Type: buildv1.GitLabWebHookBuildTriggerType,
				GitLabWebHook: &buildv1.WebHookTrigger{
					Secret: "secret201",
				},
			},
			{
				Type: buildv1.BitbucketWebHookBuildTriggerType,
				BitbucketWebHook: &buildv1.WebHookTrigger{
					Secret: "secret301",
				},
			},
		},
		CommonSpec: buildv1.CommonSpec{
			Source: buildv1.BuildSource{
				Git: &buildv1.GitBuildSource{
					URI: "git://github.com/my/repo.git",
				},
			},
			Strategy: mockBuildStrategy,
		},
	},
}
var mockBuildStrategy = buildv1.BuildStrategy{
	SourceStrategy: &buildv1.SourceBuildStrategy{
		From: corev1.ObjectReference{
			Kind: "DockerImage",
			Name: "repository/image",
		},
	},
}

func TestParseUrlError(t *testing.T) {
	responder := &fakeResponder{}
	client := newBuildConfigClient(&okBuildConfigInstantiator{})
	handler, _ := newWebHookREST(client, nil, buildv1.SchemeGroupVersion,
		map[string]webhook.Plugin{"github": github.New(), "gitlab": gitlab.New(), "bitbucket": bitbucket.New()}).
		Connect(apirequest.WithNamespace(apirequest.NewDefaultContext(), testBuildConfig.Namespace), "build100", &kapi.PodProxyOptions{Path: ""}, responder)
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
	responder := &fakeResponder{}
	client := newBuildConfigClient(&okBuildConfigInstantiator{}, testBuildConfig)
	handler, _ := newWebHookREST(client, nil, buildv1.SchemeGroupVersion, map[string]webhook.Plugin{"pathplugin": &pathPlugin{}}).
		Connect(apirequest.WithNamespace(apirequest.NewDefaultContext(), testBuildConfig.Namespace), "build100", &kapi.PodProxyOptions{Path: "secret101/pathplugin"}, responder)
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
	responder := &fakeResponder{}
	client := newBuildConfigClient(&okBuildConfigInstantiator{}, testBuildConfig)
	handler, _ := newWebHookREST(client, nil, buildv1.SchemeGroupVersion, map[string]webhook.Plugin{"pathplugin": plugin}).
		Connect(apirequest.WithNamespace(apirequest.NewDefaultContext(), testBuildConfig.Namespace), "build100", &kapi.PodProxyOptions{Path: "secret101/pathplugin/some/more/args"}, responder)
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
	responder := &fakeResponder{}
	client := newBuildConfigClient(&okBuildConfigInstantiator{}, testBuildConfig)
	handler, _ := newWebHookREST(client, nil, buildv1.SchemeGroupVersion, map[string]webhook.Plugin{"pathplugin": &pathPlugin{}}).Connect(apirequest.WithNamespace(apirequest.NewDefaultContext(),
		testBuildConfig.Namespace), "build100", &kapi.PodProxyOptions{Path: "secret101/missingplugin"}, responder)
	server := httptest.NewServer(handler)
	defer server.Close()

	_, err := http.Post(server.URL, "application/json", nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !responder.called ||
		!strings.Contains(responder.err.Error(), `buildconfighook.build.openshift.io "missingplugin" not found`) {
		t.Errorf("Expected BadRequest, got %s, expected error %s!", responder.err.Error(), `buildconfighook.build.openshift.io "missingplugin" not found`)
	}
}

func TestInvokeWebhookErrorBuildConfigInstantiate(t *testing.T) {
	responder := &fakeResponder{}
	client := newBuildConfigClient(&errorBuildConfigInstantiator{}, testBuildConfig)
	handler, _ := newWebHookREST(client, nil, buildv1.SchemeGroupVersion, map[string]webhook.Plugin{"pathplugin": &pathPlugin{}}).
		Connect(apirequest.WithNamespace(apirequest.NewDefaultContext(), testBuildConfig.Namespace), "build100", &kapi.PodProxyOptions{Path: "secret101/pathplugin"}, responder)
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
	responder := &fakeResponder{}
	client := newBuildConfigClient(&okBuildConfigInstantiator{}, testBuildConfig)
	handler, _ := newWebHookREST(client, nil, buildv1.SchemeGroupVersion, map[string]webhook.Plugin{"pathplugin": &pathPlugin{}}).
		Connect(apirequest.WithNamespace(apirequest.NewDefaultContext(), testBuildConfig.Namespace), "badbuild100", &kapi.PodProxyOptions{Path: "secret101/pathplugin"}, responder)
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
	responder := &fakeResponder{}
	client := newBuildConfigClient(&okBuildConfigInstantiator{}, testBuildConfig)
	handler, _ := newWebHookREST(client, nil, buildv1.SchemeGroupVersion, map[string]webhook.Plugin{"errPlugin": &errPlugin{}}).
		Connect(apirequest.WithNamespace(apirequest.NewDefaultContext(), testBuildConfig.Namespace), "build100", &kapi.PodProxyOptions{Path: "secret101/errPlugin"}, responder)
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
	revision := &buildv1.SourceRevision{
		Git: &buildv1.GitSourceRevision{
			Author: buildv1.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Committer: buildv1.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Message: "A random act of kindness",
		},
	}

	externalRevision := &buildv1.SourceRevision{}
	if err := legacyscheme.Scheme.Convert(revision, externalRevision, nil); err != nil {
		panic(err)
	}

	buildtriggerCause := webhook.GenerateBuildTriggerInfo(externalRevision, "generic")
	hiddenSecret := "<secret>"
	for _, cause := range buildtriggerCause {
		if !reflect.DeepEqual(externalRevision, cause.GenericWebHook.Revision) {
			t.Errorf("Expected returned revision to equal: %v", revision)
		}
		if cause.GenericWebHook.Secret != hiddenSecret {
			t.Errorf("Expected obfuscated secret to be: %s", hiddenSecret)
		}
		if cause.Message != apiserverbuildutil.BuildTriggerCauseGenericMsg {
			t.Errorf("Expected build reason to be 'Generic WebHook, go %s'", cause.Message)
		}
	}
}

func TestGeneratedBuildTriggerInfoGitHubWebHook(t *testing.T) {
	revision := &buildv1.SourceRevision{
		Git: &buildv1.GitSourceRevision{
			Author: buildv1.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Committer: buildv1.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Message: "A random act of kindness",
		},
	}
	externalRevision := &buildv1.SourceRevision{}
	if err := legacyscheme.Scheme.Convert(revision, externalRevision, nil); err != nil {
		panic(err)
	}

	buildtriggerCause := webhook.GenerateBuildTriggerInfo(externalRevision, "github")
	hiddenSecret := "<secret>"
	for _, cause := range buildtriggerCause {
		if !reflect.DeepEqual(externalRevision, cause.GitHubWebHook.Revision) {
			t.Errorf("Expected returned revision to equal: %v", revision)
		}
		if cause.GitHubWebHook.Secret != hiddenSecret {
			t.Errorf("Expected obfuscated secret to be: %s", hiddenSecret)
		}
		if cause.Message != apiserverbuildutil.BuildTriggerCauseGithubMsg {
			t.Errorf("Expected build reason to be 'GitHub WebHook, go %s'", cause.Message)
		}
	}
}

func TestGeneratedBuildTriggerInfoGitLabWebHook(t *testing.T) {
	revision := &buildv1.SourceRevision{
		Git: &buildv1.GitSourceRevision{
			Author: buildv1.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Committer: buildv1.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Message: "A random act of kindness",
		},
	}
	externalRevision := &buildv1.SourceRevision{}
	if err := legacyscheme.Scheme.Convert(revision, externalRevision, nil); err != nil {
		panic(err)
	}

	buildtriggerCause := webhook.GenerateBuildTriggerInfo(externalRevision, "gitlab")
	hiddenSecret := "<secret>"
	for _, cause := range buildtriggerCause {
		if !reflect.DeepEqual(externalRevision, cause.GitLabWebHook.Revision) {
			t.Errorf("Expected returned revision to equal: %v", revision)
		}
		if cause.GitLabWebHook.Secret != hiddenSecret {
			t.Errorf("Expected obfuscated secret to be: %s", hiddenSecret)
		}
		if cause.Message != apiserverbuildutil.BuildTriggerCauseGitLabMsg {
			t.Errorf("Expected build reason to be 'GitLab WebHook, go %s'", cause.Message)
		}
	}
}

func TestGeneratedBuildTriggerInfoBitbucketWebHook(t *testing.T) {
	revision := &buildv1.SourceRevision{
		Git: &buildv1.GitSourceRevision{
			Author: buildv1.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Committer: buildv1.SourceControlUser{
				Name:  "John Doe",
				Email: "john.doe@test.com",
			},
			Message: "A random act of kindness",
		},
	}
	externalRevision := &buildv1.SourceRevision{}
	if err := legacyscheme.Scheme.Convert(revision, externalRevision, nil); err != nil {
		panic(err)
	}

	buildtriggerCause := webhook.GenerateBuildTriggerInfo(externalRevision, "bitbucket")
	hiddenSecret := "<secret>"
	for _, cause := range buildtriggerCause {
		if !reflect.DeepEqual(externalRevision, cause.BitbucketWebHook.Revision) {
			t.Errorf("Expected returned revision to equal: %v", revision)
		}
		if cause.BitbucketWebHook.Secret != hiddenSecret {
			t.Errorf("Expected obfuscated secret to be: %s", hiddenSecret)
		}
		if cause.Message != apiserverbuildutil.BuildTriggerCauseBitbucketMsg {
			t.Errorf("Expected build reason to be 'Bitbucket WebHook, go %s'", cause.Message)
		}
	}
}
