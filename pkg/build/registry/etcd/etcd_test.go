package etcd

import (
	"reflect"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta1"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools/etcdtest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/build/api"

	"github.com/coreos/go-etcd/etcd"
)

func NewTestEtcd(client tools.EtcdClient) *Etcd {
	return New(tools.NewEtcdHelper(client, latest.Codec, etcdtest.PathPrefix()))
}

// This copy and paste is not pure ignorance.  This is that we can be sure that the key is getting made as we
// expect it to. If someone changes the location of these resources by say moving all the resources to
// "/origin/resources" (which is a really good idea), then they've made a breaking change and something should
// fail to let them know they've change some significant change and that other dependent pieces may break.
func makeTestBuildListKey(namespace string) string {
	if len(namespace) != 0 {
		return "/builds/" + namespace
	}
	return "/builds"
}
func makeTestBuildKey(namespace, id string) string {
	return "/builds/" + namespace + "/" + id
}
func makeTestDefaultBuildKey(id string) string {
	return makeTestBuildKey(kapi.NamespaceDefault, id)
}
func makeTestDefaultBuildListKey() string {
	return makeTestBuildListKey(kapi.NamespaceDefault)
}
func makeTestBuildConfigListKey(namespace string) string {
	if len(namespace) != 0 {
		return "/buildconfigs/" + namespace
	}
	return "/buildconfigs"
}
func makeTestBuildConfigKey(namespace, id string) string {
	return "/buildconfigs/" + namespace + "/" + id
}
func makeTestDefaultBuildConfigKey(id string) string {
	return makeTestBuildConfigKey(kapi.NamespaceDefault, id)
}
func makeTestDefaultBuildConfigListKey() string {
	return makeTestBuildConfigListKey(kapi.NamespaceDefault)
}

func TestEtcdGetBuild(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set(makeTestDefaultBuildKey("foo"), runtime.EncodeOrDie(latest.Codec, &api.Build{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	build, err := registry.GetBuild(kapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	} else if build.Name != "foo" {
		t.Errorf("Unexpected build: %#v", build)
	}
}

func TestEtcdGetBuildWithDuration(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	start := util.Date(2015, time.April, 8, 12, 1, 1, 0, time.Local)
	completion := util.Date(2015, time.April, 8, 12, 1, 5, 0, time.Local)
	fakeClient.Set(makeTestDefaultBuildKey("foo"), runtime.EncodeOrDie(latest.Codec, &api.Build{
		ObjectMeta:          kapi.ObjectMeta{Name: "foo"},
		StartTimestamp:      &start,
		CompletionTimestamp: &completion,
	}), 0)
	registry := NewTestEtcd(fakeClient)
	build, err := registry.GetBuild(kapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	} else if build.Name != "foo" && build.Duration != time.Duration(4)*time.Second {
		t.Errorf("Unexpected build: %#v", build)
	}
}

func TestEtcdGetBuildNotFound(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultBuildKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	_, err := registry.GetBuild(kapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Errorf("Unexpected non-error.")
	}
}

func TestEtcdCreateBuild(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.Data[makeTestDefaultBuildKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateBuild(kapi.NewDefaultContext(), &api.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
			Labels: map[string]string{
				"name": "dataBuild",
			},
		},
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Git: &api.GitBuildSource{
					URI: "http://my.build.com/the/build/Dockerfile",
				},
			},
			Strategy: api.BuildStrategy{
				Type: api.SourceBuildStrategyType,
				SourceStrategy: &api.SourceBuildStrategy{
					From: &kapi.ObjectReference{
						Name: "builder:image",
					},
				},
			},
			Output: api.BuildOutput{
				DockerImageReference: "repository/dataBuild",
			},
		},
		Status: api.BuildStatusPending,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get(makeTestDefaultBuildKey("foo"), false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var build api.Build
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &build)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if build.Name != "foo" {
		t.Errorf("Unexpected build: %#v %s", build, resp.Node.Value)
	}
}

func TestEtcdCreateBuildUsingImage(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.Data[makeTestDefaultBuildKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateBuild(kapi.NewDefaultContext(), &api.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
			Labels: map[string]string{
				"name": "dataBuild",
			},
		},
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Git: &api.GitBuildSource{
					URI: "http://my.build.com/the/build/Dockerfile",
				},
			},
			Strategy: api.BuildStrategy{
				Type: api.SourceBuildStrategyType,
				SourceStrategy: &api.SourceBuildStrategy{
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "builder/image",
					},
				},
			},
			Output: api.BuildOutput{
				DockerImageReference: "repository/dataBuild",
			},
		},
		Status: api.BuildStatusPending,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get(makeTestDefaultBuildKey("foo"), false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var build api.Build
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &build)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if build.Name != "foo" {
		t.Errorf("Unexpected build: %#v %s", build, resp.Node.Value)
	}
}

func TestEtcdCreateBuildAlreadyExisting(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultBuildKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.Build{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateBuild(kapi.NewDefaultContext(), &api.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	})
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteBuild(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true

	key := makeTestDefaultBuildKey("foo")
	fakeClient.Set(key, runtime.EncodeOrDie(latest.Codec, &api.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
	}), 0)
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteBuild(kapi.NewDefaultContext(), "foo")
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
	key := makeTestDefaultBuildListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	builds, err := registry.ListBuilds(kapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(builds.Items) != 0 {
		t.Errorf("Unexpected build list: %#v", builds)
	}
}

func TestEtcdListBuilds(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultBuildListKey()
	start := util.Date(2015, time.April, 8, 12, 1, 1, 0, time.Local)
	completion := util.Date(2015, time.April, 8, 12, 1, 5, 0, time.Local)
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Build{
							ObjectMeta:          kapi.ObjectMeta{Name: "foo"},
							StartTimestamp:      &start,
							CompletionTimestamp: &completion,
						}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Build{
							ObjectMeta:          kapi.ObjectMeta{Name: "bar"},
							StartTimestamp:      &start,
							CompletionTimestamp: &completion,
						}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	builds, err := registry.ListBuilds(kapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	duration := time.Duration(4) * time.Second
	if len(builds.Items) != 2 || builds.Items[0].Name != "foo" || builds.Items[1].Name != "bar" ||
		builds.Items[0].Duration != duration || builds.Items[1].Duration != duration {
		t.Errorf("Unexpected build list: %#v", builds)
	}
}

func TestEtcdWatchBuilds(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	filterFields := fields.SelectorFromSet(fields.Set{"metadata.name": "foo", "status": string(api.BuildStatusRunning), "podName": "foo-build"})

	watching, err := registry.WatchBuilds(kapi.NewContext(), labels.Everything(), filterFields, "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fakeClient.WaitForWatchCompletion()

	repo := &api.Build{ObjectMeta: kapi.ObjectMeta{Name: "foo"}, Status: api.BuildStatusRunning}
	repoBytes, _ := latest.Codec.Encode(repo)
	fakeClient.WatchResponse <- &etcd.Response{
		Action: "set",
		Node: &etcd.Node{
			Value: string(repoBytes),
		},
	}

	event := <-watching.ResultChan()
	if e, a := watch.Added, event.Type; e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
	if e, a := repo, event.Object; !reflect.DeepEqual(e, a) {
		t.Errorf("Expected %v, got %v", e, a)
	}

	select {
	case _, ok := <-watching.ResultChan():
		if !ok {
			t.Errorf("watching channel should be open")
		}
	default:
	}

	fakeClient.WatchInjectError <- nil
	if _, ok := <-watching.ResultChan(); ok {
		t.Errorf("watching channel should be closed")
	}
	watching.Stop()
}

func TestEtcdGetBuildConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set(makeTestDefaultBuildConfigKey("foo"), runtime.EncodeOrDie(latest.Codec, &api.BuildConfig{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	buildConfig, err := registry.GetBuildConfig(kapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	} else if buildConfig.Name != "foo" {
		t.Errorf("Unexpected BuildConfig: %#v", buildConfig)
	}
}

func TestEtcdGetBuildConfigNotFound(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultBuildConfigKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	_, err := registry.GetBuildConfig(kapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Errorf("Unexpected non-error.")
	}
}

func TestEtcdCreateBuildConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.Data[makeTestDefaultBuildConfigKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateBuildConfig(kapi.NewDefaultContext(), &api.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
			Labels: map[string]string{
				"name": "dataBuildConfig",
			},
		},
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Git: &api.GitBuildSource{
					URI: "http://my.build.com/the/build/Dockerfile",
				},
			},
			Strategy: api.BuildStrategy{
				Type: api.SourceBuildStrategyType,
				SourceStrategy: &api.SourceBuildStrategy{
					From: &kapi.ObjectReference{
						Name: "builder:image",
					},
				},
			},
			Output: api.BuildOutput{
				DockerImageReference: "repository/dataBuild",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get(makeTestDefaultBuildConfigKey("foo"), false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var buildConfig api.BuildConfig
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &buildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if buildConfig.Name != "foo" {
		t.Errorf("Unexpected buildConfig: %#v %s", buildConfig, resp.Node.Value)
	}
}

func TestEtcdCreateBuildConfigUsingImage(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.Data[makeTestDefaultBuildConfigKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateBuildConfig(kapi.NewDefaultContext(), &api.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
			Labels: map[string]string{
				"name": "dataBuildConfig",
			},
		},
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Git: &api.GitBuildSource{
					URI: "http://my.build.com/the/build/Dockerfile",
				},
			},
			Strategy: api.BuildStrategy{
				Type: api.SourceBuildStrategyType,
				SourceStrategy: &api.SourceBuildStrategy{
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "builder/image",
					},
				},
			},
			Output: api.BuildOutput{
				DockerImageReference: "repository/dataBuild",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get(makeTestDefaultBuildConfigKey("foo"), false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var buildConfig api.BuildConfig
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &buildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if buildConfig.Name != "foo" {
		t.Errorf("Unexpected buildConfig: %#v %s", buildConfig, resp.Node.Value)
	}
}

func TestEtcdCreateBuildConfigAlreadyExisting(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultBuildConfigKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.BuildConfig{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateBuildConfig(kapi.NewDefaultContext(), &api.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	})
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteBuildConfig(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true

	key := makeTestDefaultBuildConfigKey("foo")
	fakeClient.Set(key, runtime.EncodeOrDie(latest.Codec, &api.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},
	}), 0)
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteBuildConfig(kapi.NewDefaultContext(), "foo")
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
	key := makeTestDefaultBuildConfigListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	buildConfigs, err := registry.ListBuildConfigs(kapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(buildConfigs.Items) != 0 {
		t.Errorf("Unexpected buildConfig list: %#v", buildConfigs)
	}
}

func TestEtcdListBuildConfigs(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultBuildConfigListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.BuildConfig{
							ObjectMeta: kapi.ObjectMeta{Name: "foo"},
						}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.BuildConfig{
							ObjectMeta: kapi.ObjectMeta{Name: "bar"},
						}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	buildConfigs, err := registry.ListBuildConfigs(kapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(buildConfigs.Items) != 2 || buildConfigs.Items[0].Name != "foo" || buildConfigs.Items[1].Name != "bar" {
		t.Errorf("Unexpected buildConfig list: %#v", buildConfigs)
	}
}

func TestEtcdCreateBuildConfigFailsWithoutNamespace(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateBuildConfig(kapi.NewContext(), &api.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	})

	if err == nil {
		t.Errorf("expected error that namespace was missing from context")
	}
}

func TestEtcdCreateBuildFailsWithoutNamespace(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateBuild(kapi.NewContext(), &api.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	})

	if err == nil {
		t.Errorf("expected error that namespace was missing from context")
	}
}

func TestEtcdListBuildsInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kapi.WithNamespace(kapi.NewContext(), "alfa")
	namespaceBravo := kapi.WithNamespace(kapi.NewContext(), "bravo")
	fakeClient.Data["/builds/alfa"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Build{ObjectMeta: kapi.ObjectMeta{Name: "foo1"}}),
					},
				},
			},
		},
		E: nil,
	}
	fakeClient.Data["/builds/bravo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Build{ObjectMeta: kapi.ObjectMeta{Name: "foo2"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Build{ObjectMeta: kapi.ObjectMeta{Name: "bar2"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)

	buildsAlfa, err := registry.ListBuilds(namespaceAlfa, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(buildsAlfa.Items) != 1 || buildsAlfa.Items[0].Name != "foo1" {
		t.Errorf("Unexpected builds list: %#v", buildsAlfa)
	}

	buildsBravo, err := registry.ListBuilds(namespaceBravo, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(buildsBravo.Items) != 2 || buildsBravo.Items[0].Name != "foo2" || buildsBravo.Items[1].Name != "bar2" {
		t.Errorf("Unexpected builds list: %#v", buildsBravo)
	}
}

func TestEtcdListBuildConfigsInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kapi.WithNamespace(kapi.NewContext(), "alfa")
	namespaceBravo := kapi.WithNamespace(kapi.NewContext(), "bravo")
	fakeClient.Data["/buildconfigs/alfa"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.BuildConfig{ObjectMeta: kapi.ObjectMeta{Name: "foo1"}}),
					},
				},
			},
		},
		E: nil,
	}
	fakeClient.Data["/buildconfigs/bravo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.BuildConfig{ObjectMeta: kapi.ObjectMeta{Name: "foo2"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.BuildConfig{ObjectMeta: kapi.ObjectMeta{Name: "bar2"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)

	buildConfigsAlfa, err := registry.ListBuildConfigs(namespaceAlfa, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(buildConfigsAlfa.Items) != 1 || buildConfigsAlfa.Items[0].Name != "foo1" {
		t.Errorf("Unexpected builds list: %#v", buildConfigsAlfa)
	}

	buildConfigsBravo, err := registry.ListBuildConfigs(namespaceBravo, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(buildConfigsBravo.Items) != 2 || buildConfigsBravo.Items[0].Name != "foo2" || buildConfigsBravo.Items[1].Name != "bar2" {
		t.Errorf("Unexpected builds list: %#v", buildConfigsBravo)
	}
}

func TestEtcdGetBuildConfigInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kapi.WithNamespace(kapi.NewContext(), "alfa")
	namespaceBravo := kapi.WithNamespace(kapi.NewContext(), "bravo")
	fakeClient.Set("/buildconfigs/alfa/foo", runtime.EncodeOrDie(latest.Codec, &api.BuildConfig{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	fakeClient.Set("/buildconfigs/bravo/foo", runtime.EncodeOrDie(latest.Codec, &api.BuildConfig{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)

	alfaFoo, err := registry.GetBuildConfig(namespaceAlfa, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if alfaFoo == nil || alfaFoo.Name != "foo" {
		t.Errorf("Unexpected buildConfig: %#v", alfaFoo)
	}

	bravoFoo, err := registry.GetBuildConfig(namespaceBravo, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if bravoFoo == nil || bravoFoo.Name != "foo" {
		t.Errorf("Unexpected buildConfig: %#v", bravoFoo)
	}
}

func TestEtcdGetBuildInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kapi.WithNamespace(kapi.NewContext(), "alfa")
	namespaceBravo := kapi.WithNamespace(kapi.NewContext(), "bravo")
	fakeClient.Set("/builds/alfa/foo", runtime.EncodeOrDie(latest.Codec, &api.Build{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	fakeClient.Set("/builds/bravo/foo", runtime.EncodeOrDie(latest.Codec, &api.Build{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)

	alfaFoo, err := registry.GetBuild(namespaceAlfa, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if alfaFoo == nil || alfaFoo.Name != "foo" {
		t.Errorf("Unexpected buildConfig: %#v", alfaFoo)
	}

	bravoFoo, err := registry.GetBuild(namespaceBravo, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if bravoFoo == nil || bravoFoo.Name != "foo" {
		t.Errorf("Unexpected buildConfig: %#v", bravoFoo)
	}
}
