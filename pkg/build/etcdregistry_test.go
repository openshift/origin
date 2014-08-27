package build

import (
	//	"reflect"
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	//	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta1"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/build/api/v1beta1"

	"github.com/coreos/go-etcd/etcd"
)

func NewTestEtcdRegistry(client tools.EtcdClient) *EtcdRegistry {
	registry := NewEtcdRegistry(client)
	return registry
}

func TestEtcdGetBuild(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set("/registry/builds/foo", runtime.EncodeOrDie(api.Build{JSONBase: kubeapi.JSONBase{ID: "foo"}}), 0)
	registry := NewTestEtcdRegistry(fakeClient)
	build, err := registry.GetBuild("foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	} else if build.ID != "foo" {
		t.Errorf("Unexpected build: %#v", build)
	}
}

func TestEtcdGetBuildNotFound(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data["/registry/builds/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcdRegistry(fakeClient)
	_, err := registry.GetBuild("foo")
	if err == nil {
		t.Errorf("Unexpected non-error.")
	}
}

func TestEtcdCreateBuild(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.Data["/registry/builds/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcdRegistry(fakeClient)
	err := registry.CreateBuild(&api.Build{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
		},
		Input: api.BuildInput{
			Type:      api.DockerBuildType,
			SourceURI: "http://my.build.com/the/build/Dockerfile",
			ImageTag:  "repository/dataBuild",
		},
		Status: api.BuildPending,
		PodID:  "-the-pod-id",
		Labels: map[string]string{
			"name": "dataBuild",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get("/registry/builds/foo", false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var build api.Build
	err = runtime.DecodeInto([]byte(resp.Node.Value), &build)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if build.ID != "foo" {
		t.Errorf("Unexpected build: %#v %s", build, resp.Node.Value)
	}
}

func TestEtcdCreateBuildAlreadyExisting(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data["/registry/builds/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(api.Build{JSONBase: kubeapi.JSONBase{ID: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcdRegistry(fakeClient)
	err := registry.CreateBuild(&api.Build{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
		},
	})
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteBuild(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true

	key := "/registry/builds/foo"
	fakeClient.Set(key, runtime.EncodeOrDie(api.Build{
		JSONBase: kubeapi.JSONBase{ID: "foo"},
	}), 0)
	registry := NewTestEtcdRegistry(fakeClient)
	err := registry.DeleteBuild("foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(fakeClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeClient.DeletedKeys)
	} else if fakeClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeClient.DeletedKeys[0], key)
	}
}

func TestEtcdEmptyListBuilds(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/registry/builds"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcdRegistry(fakeClient)
	builds, err := registry.ListBuilds(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(builds.Items) != 0 {
		t.Errorf("Unexpected build list: %#v", builds)
	}
}

func TestEtcdListBuilds(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/registry/builds"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(api.Build{
							JSONBase: kubeapi.JSONBase{ID: "foo"},
						}),
					},
					{
						Value: runtime.EncodeOrDie(api.Build{
							JSONBase: kubeapi.JSONBase{ID: "bar"},
						}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcdRegistry(fakeClient)
	builds, err := registry.ListBuilds(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(builds.Items) != 2 || builds.Items[0].ID != "foo" || builds.Items[1].ID != "bar" {
		t.Errorf("Unexpected build list: %#v", builds)
	}
}

func TestEtcdGetBuildConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set("/registry/build-configs/foo", runtime.EncodeOrDie(api.BuildConfig{JSONBase: kubeapi.JSONBase{ID: "foo"}}), 0)
	registry := NewTestEtcdRegistry(fakeClient)
	buildConfig, err := registry.GetBuildConfig("foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	} else if buildConfig.ID != "foo" {
		t.Errorf("Unexpected build config: %#v", buildConfig)
	}
}

func TestEtcdGetBuildConfigNotFound(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data["/registry/build-configs/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcdRegistry(fakeClient)
	_, err := registry.GetBuildConfig("foo")
	if err == nil {
		t.Errorf("Unexpected non-error.")
	}
}

func TestEtcdCreateBuildConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.Data["/registry/build-configs/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcdRegistry(fakeClient)
	err := registry.CreateBuildConfig(&api.BuildConfig{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
		},
		DesiredInput: api.BuildInput{
			Type:      api.DockerBuildType,
			SourceURI: "http://my.build.com/the/build/Dockerfile",
			ImageTag:  "repository/dataBuild",
		},
		Labels: map[string]string{
			"name": "dataBuildConfig",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get("/registry/build-configs/foo", false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var buildConfig api.BuildConfig
	err = runtime.DecodeInto([]byte(resp.Node.Value), &buildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if buildConfig.ID != "foo" {
		t.Errorf("Unexpected buildConfig: %#v %s", buildConfig, resp.Node.Value)
	}
}

func TestEtcdCreateBuildConfigAlreadyExisting(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data["/registry/build-configs/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(api.BuildConfig{JSONBase: kubeapi.JSONBase{ID: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcdRegistry(fakeClient)
	err := registry.CreateBuildConfig(&api.BuildConfig{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
		},
	})
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteBuildConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true

	key := "/registry/build-configs/foo"
	fakeClient.Set(key, runtime.EncodeOrDie(api.BuildConfig{
		JSONBase: kubeapi.JSONBase{ID: "foo"},
	}), 0)
	registry := NewTestEtcdRegistry(fakeClient)
	err := registry.DeleteBuildConfig("foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(fakeClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeClient.DeletedKeys)
	} else if fakeClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeClient.DeletedKeys[0], key)
	}
}

func TestEtcdEmptyListBuildConfigs(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/registry/build-configs"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcdRegistry(fakeClient)
	buildConfigs, err := registry.ListBuildConfigs(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(buildConfigs.Items) != 0 {
		t.Errorf("Unexpected buildConfig list: %#v", buildConfigs)
	}
}

func TestEtcdListBuildConfigs(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/registry/build-configs"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(api.BuildConfig{
							JSONBase: kubeapi.JSONBase{ID: "foo"},
						}),
					},
					{
						Value: runtime.EncodeOrDie(api.BuildConfig{
							JSONBase: kubeapi.JSONBase{ID: "bar"},
						}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcdRegistry(fakeClient)
	buildConfigs, err := registry.ListBuildConfigs(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(buildConfigs.Items) != 2 || buildConfigs.Items[0].ID != "foo" || buildConfigs.Items[1].ID != "bar" {
		t.Errorf("Unexpected buildConfig list: %#v", buildConfigs)
	}
}
