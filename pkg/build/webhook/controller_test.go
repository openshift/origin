package webhook

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/build/api"
)

type okBuildConfigGetter struct{}

func (*okBuildConfigGetter) Get(namespace, name string) (*api.BuildConfig, error) {
	return &api.BuildConfig{
		Parameters: api.BuildParameters{
			Strategy: api.BuildStrategy{
				Type: "STI",
				SourceStrategy: &api.SourceBuildStrategy{
					From: kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "repository/builder-image",
					},
				},
			},
		},
		Triggers: []api.BuildTriggerPolicy{
			{
				Type: api.GitHubWebHookBuildTriggerType,
				GitHubWebHook: &api.WebHookTrigger{
					Secret: "secret101",
				},
			},
		},
	}, nil
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

func (p *pathPlugin) Extract(buildCfg *api.BuildConfig, secret, path string, req *http.Request) (*api.SourceRevision, bool, error) {
	p.Path = path
	return nil, true, nil
}

type errPlugin struct{}

func (*errPlugin) Extract(buildCfg *api.BuildConfig, secret, path string, req *http.Request) (*api.SourceRevision, bool, error) {
	return nil, true, errors.New("Plugin error!")
}

func TestParseUrlError(t *testing.T) {
	server := httptest.NewServer(NewController(&okBuildConfigGetter{}, &okBuildConfigInstantiator{},
		nil))
	defer server.Close()

	resp, err := http.Post(server.URL, "application/json", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Wrong response code, expecting 404, got %s: %s!", resp.Status,
			string(body))
	}
}

func TestParseUrlOK(t *testing.T) {
	server := httptest.NewServer(NewController(&okBuildConfigGetter{}, &okBuildConfigInstantiator{},
		map[string]Plugin{
			"pathplugin": &pathPlugin{},
		}))
	defer server.Close()

	resp, err := http.Post(server.URL+"/build100/secret101/pathplugin",
		"application/json", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Wrong response code, expecting 200, got %s: %s!", resp.Status,
			string(body))
	}
}

func TestParseUrlLong(t *testing.T) {
	plugin := &pathPlugin{}
	server := httptest.NewServer(NewController(&okBuildConfigGetter{}, &okBuildConfigInstantiator{},
		map[string]Plugin{
			"pathplugin": plugin,
		}))
	defer server.Close()

	resp, err := http.Post(server.URL+"/build100/secret101/pathplugin/some/more/args",
		"application/json", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Wrong response code, expecting 200, got %s: %s!", resp.Status,
			string(body))
	}
	if plugin.Path != "some/more/args" {
		t.Errorf("Expected some/more/args got %s!", plugin.Path)
	}
}

func TestInvokeWebhookErrorSecret(t *testing.T) {
	server := httptest.NewServer(NewController(&okBuildConfigGetter{}, &okBuildConfigInstantiator{},
		nil))
	defer server.Close()

	resp, err := http.Post(server.URL+"/build100/wrongsecret/somePlugin",
		"application/json", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest && len(body) == 0 {
		t.Errorf("Wrong response code, expecting 400, got %s: %s!", resp.Status,
			string(body))
	}
}

func TestInvokeWebhookMissingPlugin(t *testing.T) {
	server := httptest.NewServer(NewController(&okBuildConfigGetter{}, &okBuildConfigInstantiator{},
		nil))
	defer server.Close()

	resp, err := http.Post(server.URL+"/build100/secret101/missingplugin",
		"application/json", nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNotFound &&
		!strings.Contains(string(body), "Plugin missingplugin not found") {
		t.Errorf("Wrong response code, expecting 400, got %s: %s!", resp.Status,
			string(body))
	}
}

func TestInvokeWebhookErrorBuildConfig(t *testing.T) {
	server := httptest.NewServer(NewController(&okBuildConfigGetter{}, &errorBuildConfigInstantiator{},
		map[string]Plugin{
			"okPlugin": &pathPlugin{},
		}))
	defer server.Close()

	resp, err := http.Post(server.URL+"/build100/secret101/okPlugin",
		"application/json", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest &&
		!strings.Contains(string(body), "Build error!") {
		t.Errorf("Wrong response code, expecting 400, got %s: %s!", resp.Status,
			string(body))
	}
}

func TestInvokeWebhookErrorGetConfig(t *testing.T) {
	server := httptest.NewServer(NewController(&errorBuildConfigGetter{}, &okBuildConfigInstantiator{},
		nil))
	defer server.Close()

	resp, err := http.Post(server.URL+"/build100/secret101/errPlugin",
		"application/json", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest &&
		!strings.Contains(string(body), "BuildConfig error!") {
		t.Errorf("Wrong response code, expecting 400, got %s: %s!", resp.Status,
			string(body))
	}
}

func TestInvokeWebhookErrorCreateBuild(t *testing.T) {
	server := httptest.NewServer(NewController(&okBuildConfigGetter{}, &okBuildConfigInstantiator{},
		map[string]Plugin{
			"errPlugin": &errPlugin{},
		}))
	defer server.Close()

	resp, err := http.Post(server.URL+"/build100/secret101/errPlugin",
		"application/json", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest &&
		!strings.Contains(string(body), "Plugin error!") {
		t.Errorf("Wrong response code, expecting 400, got %s: %s!", resp.Status,
			string(body))
	}
}

type mockOkBuildConfigInstantiator struct {
	testBuildInterface
}
type testBuildInterface struct {
	InstantiateFunc func(namespace string, request *api.BuildRequest) (*api.Build, error)
}

func (i *testBuildInterface) Instantiate(namespace string, request *api.BuildRequest) (*api.Build, error) {
	return i.InstantiateFunc(namespace, request)
}

type mockOkBuildConfigGetter struct {
	testBuildConfigInterface
}
type testBuildConfigInterface struct {
	GetBuildConfigFunc func(namespace, name string) (*api.BuildConfig, error)
}

func (i *testBuildConfigInterface) Get(namespace, name string) (*api.BuildConfig, error) {
	return i.GetBuildConfigFunc(namespace, name)
}

func TestInvokeWebhookOK(t *testing.T) {
	var buildRequest string
	buildConfig := &api.BuildConfig{
		Parameters: api.BuildParameters{
			Strategy: api.BuildStrategy{
				Type: "STI",
				SourceStrategy: &api.SourceBuildStrategy{
					From: kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "repository/builder-image",
					},
				},
			},
		},
	}

	server := httptest.NewServer(NewController(
		&mockOkBuildConfigGetter{
			testBuildConfigInterface: testBuildConfigInterface{
				GetBuildConfigFunc: func(namespace, name string) (*api.BuildConfig, error) {
					buildConfig.Name = name
					return buildConfig, nil
				},
			},
		},
		&mockOkBuildConfigInstantiator{
			testBuildInterface: testBuildInterface{
				InstantiateFunc: func(namespace string, request *api.BuildRequest) (*api.Build, error) {
					buildRequest = request.Name
					return &api.Build{}, nil
				},
			},
		},

		map[string]Plugin{
			"okPlugin": &pathPlugin{},
		}))
	defer server.Close()

	resp, err := http.Post(server.URL+"/build100/secret101/okPlugin",
		"application/json", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Wrong response code, expecting 200, got %s: %s!", resp.Status,
			string(body))
	}
	if buildConfig.Name != buildRequest {
		t.Fatalf("expected buildconfig names to match '%s', got '%s'", buildConfig.Name, buildRequest)
	}
}
