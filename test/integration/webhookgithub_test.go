// +build integration,!no-etcd

package integration

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/version"

	"github.com/openshift/origin/pkg/build"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/github"
	osclient "github.com/openshift/origin/pkg/client"
)

func init() {
	requireEtcd()
}

func TestWebhookGithubPush(t *testing.T) {
	etcdClient := newEtcdClient()
	m := master.New(&master.Config{
		EtcdServers: etcdClient.GetCluster(),
	})
	osMux := http.NewServeMux()
	storage := map[string]apiserver.RESTStorage{
		"builds":       buildregistry.NewStorage(build.NewEtcdRegistry(etcdClient)),
		"buildConfigs": buildconfigregistry.NewStorage(build.NewEtcdRegistry(etcdClient)),
	}
	apiserver.NewAPIGroup(m.API_v1beta1()).InstallREST(osMux, "/api/v1beta1")
	osPrefix := "/osapi/v1beta1"
	apiserver.NewAPIGroup(storage, runtime.Codec).InstallREST(osMux, osPrefix)
	apiserver.InstallSupport(osMux)

	s := httptest.NewServer(osMux)

	kubeclient := client.NewOrDie(s.URL, nil)
	osClient, _ := osclient.New(s.URL, nil)

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

	// create buildconfig
	buildConfig := &buildapi.BuildConfig{
		JSONBase: kubeapi.JSONBase{
			ID: "build100",
		},
		DesiredInput: buildapi.BuildInput{
			Type:      buildapi.DockerBuildType,
			SourceURI: "http://my.docker/build",
			ImageTag:  "namespace/builtimage",
		},
		Secret: "secret101",
	}
	if _, err := osClient.CreateBuildConfig(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// trigger build event sending push notification
	client := &http.Client{}
	data, err := ioutil.ReadFile("../../pkg/build/webhook/github/fixtures/pushevent.json")
	if err != nil {
		t.Fatalf("Failed to open pushevent.json: %v", err)
	}
	req, err := http.NewRequest("POST", s.URL+whPrefix+"build100/secret101/github", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Error creating POST request: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "GitHub-Hookshot/github")
	req.Header.Add("X-Github-Event", "push")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed posting webhook: %v", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Wrong response code, expecting 200, got %s: %s!",
			resp.Status, string(body))
	}

	// get a list of builds
	builds, err := osClient.ListBuilds(labels.Everything())
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

}
