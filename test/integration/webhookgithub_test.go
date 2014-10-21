// +build integration,!no-etcd

package integration

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/version"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildetcd "github.com/openshift/origin/pkg/build/registry/etcd"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/github"
	osclient "github.com/openshift/origin/pkg/client"
)

func init() {
	requireEtcd()
}

func TestWebhookGithubPush(t *testing.T) {
	ctx := kapi.NewContext()
	osClient, url := setup(t)

	// create buildconfig
	buildConfig := &buildapi.BuildConfig{
		JSONBase: kapi.JSONBase{
			ID: "pushbuild",
		},
		DesiredInput: buildapi.BuildInput{
			Type:      buildapi.DockerBuildType,
			SourceURI: "http://my.docker/build",
			ImageTag:  "namespace/builtimage",
		},
		Secret: "secret101",
	}
	if _, err := osClient.CreateBuildConfig(ctx, buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// trigger build event sending push notification
	postFile("push", "pushevent.json", url+"pushbuild/secret101/github", http.StatusOK, t)

	// get a list of builds
	builds, err := osClient.ListBuilds(ctx, labels.Everything())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(builds.Items) != 1 {
		t.Fatalf("Expected one build, got %#v", builds)
	}
	actual := builds.Items[0]
	if actual.Status != buildapi.BuildNew {
		t.Errorf("Expected %s, got %s", buildapi.BuildNew, actual.Status)
	}
	if !reflect.DeepEqual(actual.Input, buildConfig.DesiredInput) {
		t.Errorf("Expected %#v, got %#v", buildConfig.DesiredInput, actual.Input)
	}

	cleanup(osClient, ctx, t)
}

func TestWebhookGithubPing(t *testing.T) {
	ctx := kapi.NewContext()
	osClient, url := setup(t)

	// create buildconfig
	buildConfig := &buildapi.BuildConfig{
		JSONBase: kapi.JSONBase{
			ID: "pingbuild",
		},
		DesiredInput: buildapi.BuildInput{
			Type:      buildapi.DockerBuildType,
			SourceURI: "http://my.docker/build",
			ImageTag:  "namespace/builtimage",
		},
		Secret: "secret101",
	}
	if _, err := osClient.CreateBuildConfig(ctx, buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// trigger build event sending push notification
	postFile("ping", "pingevent.json", url+"pingbuild/secret101/github", http.StatusOK, t)

	// get a list of builds
	builds, err := osClient.ListBuilds(ctx, labels.Everything())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(builds.Items) != 0 {
		t.Fatalf("Unexpected build appeared, got %#v", builds)
	}

	cleanup(osClient, ctx, t)
}

func setup(t *testing.T) (*osclient.Client, string) {
	deleteAllEtcdKeys()
	etcdClient := newEtcdClient()
	helper, _ := master.NewEtcdHelper(etcdClient.GetCluster(), klatest.Version)
	m := master.New(&master.Config{
		EtcdHelper: helper,
	})
	interfaces, _ := latest.InterfacesFor(latest.Version)
	buildRegistry := buildetcd.New(tools.EtcdHelper{etcdClient, interfaces.Codec, interfaces.ResourceVersioner})
	storage := map[string]apiserver.RESTStorage{
		"builds":       buildregistry.NewREST(buildRegistry),
		"buildConfigs": buildconfigregistry.NewREST(buildRegistry),
	}

	osMux := http.NewServeMux()
	apiserver.NewAPIGroup(m.API_v1beta1()).InstallREST(osMux, "/api/v1beta1")
	osPrefix := "/osapi/v1beta1"
	apiserver.NewAPIGroup(storage, v1beta1.Codec, osPrefix, interfaces.SelfLinker).InstallREST(osMux, osPrefix)
	apiserver.InstallSupport(osMux)
	s := httptest.NewServer(osMux)

	kubeclient := client.NewOrDie(&client.Config{Host: s.URL, Version: klatest.Version})
	osClient := osclient.NewOrDie(&client.Config{Host: s.URL, Version: latest.Version})

	whPrefix := osPrefix + "/buildConfigHooks/"
	osMux.Handle(whPrefix, http.StripPrefix(whPrefix,
		webhook.NewController(osClient, map[string]webhook.Plugin{
			"github": github.New(),
		})))

	info, err := kubeclient.ServerVersion()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if e, a := version.Get(), *info; !reflect.DeepEqual(e, a) {
		t.Errorf("Expected %#v, got %#v", e, a)
	}

	return osClient, s.URL + whPrefix
}

func cleanup(osClient *osclient.Client, ctx kapi.Context, t *testing.T) {
	builds, err := osClient.ListBuilds(ctx, labels.Everything())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	for _, build := range builds.Items {
		if err := osClient.DeleteBuild(ctx, build.ID); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}
	buildConfigs, err := osClient.ListBuildConfigs(ctx, labels.Everything())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	for _, bc := range buildConfigs.Items {
		if err := osClient.DeleteBuildConfig(ctx, bc.ID); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func postFile(event, filename, url string, expStatusCode int, t *testing.T) {
	client := &http.Client{}
	data, err := ioutil.ReadFile("../../pkg/build/webhook/github/fixtures/" + filename)
	if err != nil {
		t.Fatalf("Failed to open %s: %v", filename, err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Error creating POST request: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "GitHub-Hookshot/github")
	req.Header.Add("X-Github-Event", event)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed posting webhook: %v", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != expStatusCode {
		t.Errorf("Wrong response code, expecting %d, got %s: %s!", expStatusCode, resp.Status, string(body))
	}
}
