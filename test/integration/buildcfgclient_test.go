// +build integration,!no-etcd

package integration

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/version"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
	"github.com/openshift/origin/pkg/build/api"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildetcd "github.com/openshift/origin/pkg/build/registry/etcd"
	osclient "github.com/openshift/origin/pkg/client"
)

func init() {
	requireEtcd()
}

func TestBuildConfigClient(t *testing.T) {
	etcdClient := newEtcdClient()
	helper, _ := master.NewEtcdHelper(etcdClient.GetCluster(), klatest.Version)
	m := master.New(&master.Config{
		EtcdHelper: helper,
	})
	codec, versioner, _ := latest.InterfacesFor(latest.Version)
	buildRegistry := buildetcd.New(tools.EtcdHelper{etcdClient, codec, versioner})
	storage := map[string]apiserver.RESTStorage{
		"builds":       buildregistry.NewREST(buildRegistry),
		"buildConfigs": buildconfigregistry.NewREST(buildRegistry),
	}

	osMux := http.NewServeMux()
	apiserver.NewAPIGroup(m.API_v1beta1()).InstallREST(osMux, "/api/v1beta1")
	apiserver.NewAPIGroup(storage, v1beta1.Codec).InstallREST(osMux, "/osapi/v1beta1")
	apiserver.InstallSupport(osMux)
	s := httptest.NewServer(osMux)

	kubeclient := client.NewOrDie(s.URL, klatest.Version, nil)
	osclient, _ := osclient.New(s.URL, latest.Version, nil)

	info, err := kubeclient.ServerVersion()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := version.Get(), *info; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %#v, got %#v", e, a)
	}

	buildConfigs, err := osclient.ListBuildConfigs(labels.Everything())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(buildConfigs.Items) != 0 {
		t.Errorf("expected no buildConfigs, got %#v", buildConfigs)
	}

	// get a validation error
	buildConfig := &api.BuildConfig{
		Labels: map[string]string{
			"label1": "value1",
			"label2": "value2",
		},
		DesiredInput: api.BuildInput{
			Type:         api.DockerBuildType,
			SourceURI:    "http://my.docker/build",
			ImageTag:     "namespace/builtimage",
			BuilderImage: "anImage",
		},
	}
	got, err := osclient.CreateBuildConfig(buildConfig)
	if err == nil {
		t.Fatalf("unexpected non-error: %v", err)
	}

	// get a created buildConfig
	buildConfig.DesiredInput.BuilderImage = ""
	got, err = osclient.CreateBuildConfig(buildConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID == "" {
		t.Errorf("unexpected empty buildConfig ID %v", got)
	}

	// get a list of buildConfigs
	buildConfigs, err = osclient.ListBuildConfigs(labels.Everything())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(buildConfigs.Items) != 1 {
		t.Errorf("expected one buildConfig, got %#v", buildConfigs)
	}
	actual := buildConfigs.Items[0]
	if actual.ID != got.ID {
		t.Errorf("expected buildConfig %#v, got %#v", got, actual)
	}

	// delete a buildConfig
	err = osclient.DeleteBuildConfig(got.ID)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	buildConfigs, err = osclient.ListBuildConfigs(labels.Everything())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(buildConfigs.Items) != 0 {
		t.Errorf("expected no buildConfigs, got %#v", buildConfigs)
	}
}
