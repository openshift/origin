// +build integration,!no-etcd

package integration

import (
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
	deleteAllEtcdKeys()
	ctx := kapi.NewContext()
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
	apiserver.NewAPIGroup(storage, v1beta1.Codec, "/osapi/v1beta1", interfaces.SelfLinker).InstallREST(osMux, "/osapi/v1beta1")
	apiserver.InstallSupport(osMux)
	s := httptest.NewServer(osMux)

	kubeclient := client.NewOrDie(&client.Config{Host: s.URL, Version: klatest.Version})
	osClient := osclient.NewOrDie(&client.Config{Host: s.URL, Version: latest.Version})

	info, err := kubeclient.ServerVersion()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := version.Get(), *info; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %#v, got %#v", e, a)
	}

	buildConfigs, err := osClient.ListBuildConfigs(ctx, labels.Everything())
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
	got, err := osClient.CreateBuildConfig(ctx, buildConfig)
	if err == nil {
		t.Fatalf("unexpected non-error: %v", err)
	}

	// get a created buildConfig
	buildConfig.DesiredInput.BuilderImage = ""
	got, err = osClient.CreateBuildConfig(ctx, buildConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID == "" {
		t.Errorf("unexpected empty buildConfig ID %v", got)
	}

	// get a list of buildConfigs
	buildConfigs, err = osClient.ListBuildConfigs(ctx, labels.Everything())
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
	err = osClient.DeleteBuildConfig(ctx, got.ID)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	buildConfigs, err = osClient.ListBuildConfigs(ctx, labels.Everything())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(buildConfigs.Items) != 0 {
		t.Errorf("expected no buildConfigs, got %#v", buildConfigs)
	}
}
