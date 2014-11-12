package webhook

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
)

type osClient struct {
	client.Fake
}

func (_ *osClient) GetBuildConfig(ctx kapi.Context, id string) (result *api.BuildConfig, err error) {
	return &api.BuildConfig{
		Triggers: []api.BuildTriggerPolicy{
			{
				Type: api.GithubWebHookType,
				GithubWebHook: &api.WebHookTrigger{
					Secret: "secret101",
				},
			},
		},
	}, nil
}

func (_ *osClient) WatchBuilds(ctx kapi.Context, field, label labels.Selector, resourceVersion string) (watch.Interface, error) {
	return nil, nil
}

type buildErrorClient struct {
	osClient
}

func (_ *buildErrorClient) CreateBuild(ctx kapi.Context, build *api.Build) (result *api.Build, err error) {
	return &api.Build{}, errors.New("Build error!")
}

type configErrorClient struct {
	osClient
}

func (_ *configErrorClient) GetBuildConfig(ctx kapi.Context, id string) (result *api.BuildConfig, err error) {
	return &api.BuildConfig{}, errors.New("BuildConfig error!")
}

type pathPlugin struct {
	Path string
}

func (p *pathPlugin) Extract(buildCfg *api.BuildConfig, secret, path string, req *http.Request) (*api.Build, bool, error) {
	p.Path = path
	return nil, true, nil
}

type errPlugin struct{}

func (_ *errPlugin) Extract(buildCfg *api.BuildConfig, secret, path string, req *http.Request) (*api.Build, bool, error) {
	return nil, true, errors.New("Plugin error!")
}

func TestParseUrlError(t *testing.T) {
	server := httptest.NewServer(NewController(&osClient{}, nil))
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
	server := httptest.NewServer(NewController(&osClient{}, map[string]Plugin{
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
	server := httptest.NewServer(NewController(&osClient{}, map[string]Plugin{
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
	server := httptest.NewServer(NewController(&osClient{}, nil))
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
	server := httptest.NewServer(NewController(&osClient{}, nil))
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
	server := httptest.NewServer(NewController(&buildErrorClient{}, map[string]Plugin{
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
	server := httptest.NewServer(NewController(&configErrorClient{}, nil))
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
	server := httptest.NewServer(NewController(&osClient{}, map[string]Plugin{
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

type mockOsClient struct {
	testBuildInterface
}

type testBuildInterface struct {
	CreateBuildFunc    func(ctx kapi.Context, build *api.Build) (*api.Build, error)
	GetBuildConfigFunc func(ctx kapi.Context, id string) (*api.BuildConfig, error)
}

func (i *testBuildInterface) CreateBuild(ctx kapi.Context, build *api.Build) (*api.Build, error) {
	return i.CreateBuildFunc(ctx, build)
}
func (i *testBuildInterface) GetBuildConfig(ctx kapi.Context, id string) (*api.BuildConfig, error) {
	return i.GetBuildConfigFunc(ctx, id)
}

func TestInvokeWebhookOk(t *testing.T) {
	var buildRequest *api.Build
	buildConfig := &api.BuildConfig{
		Parameters: api.BuildParameters{},
	}

	server := httptest.NewServer(NewController(&mockOsClient{
		testBuildInterface: testBuildInterface{
			CreateBuildFunc: func(ctx kapi.Context, build *api.Build) (*api.Build, error) {
				buildRequest = build
				return build, nil
			},
			GetBuildConfigFunc: func(ctx kapi.Context, name string) (*api.BuildConfig, error) {
				buildConfig.Name = name
				return buildConfig, nil
			},
		},
	}, map[string]Plugin{
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
	if e, a := buildConfig.Name, buildRequest.Labels[api.BuildConfigLabel]; e != a {
		t.Fatalf("expected build with label '%s', got '%s'", e, a)
	}
}
