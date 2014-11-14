package etcd

import (
	"fmt"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/coreos/go-etcd/etcd"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/project/api"
)

func NewTestEtcd(client tools.EtcdClient) *Etcd {
	return New(tools.EtcdHelper{client, latest.Codec, tools.RuntimeVersionAdapter{latest.ResourceVersioner}})
}

func TestEtcdListProjectsEmpty(t *testing.T) {
	ctx := kapi.NewContext()
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeProjectListKey(ctx)
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	projects, err := registry.ListProjects(ctx, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(projects.Items) != 0 {
		t.Errorf("Unexpected projects list: %#v", projects)
	}
}

func TestEtcdListProjectsError(t *testing.T) {
	ctx := kapi.NewContext()
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeProjectListKey(ctx)
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: fmt.Errorf("some error"),
	}
	registry := NewTestEtcd(fakeClient)
	projects, err := registry.ListProjects(ctx, labels.Everything())
	if err == nil {
		t.Error("unexpected nil error")
	}

	if projects != nil {
		t.Errorf("Unexpected non-nil projects: %#v", projects)
	}
}

func TestEtcdListProjectsEverything(t *testing.T) {
	ctx := kapi.NewContext()
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeProjectListKey(ctx)
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Project{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Project{ObjectMeta: kapi.ObjectMeta{Name: "bar"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	projects, err := registry.ListProjects(ctx, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(projects.Items) != 2 || projects.Items[0].Name != "foo" || projects.Items[1].Name != "bar" {
		t.Errorf("Unexpected projects list: %#v", projects)
	}
}

func TestEtcdListProjectsFiltered(t *testing.T) {
	ctx := kapi.NewContext()
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeProjectListKey(ctx)
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Project{
							ObjectMeta: kapi.ObjectMeta{
								Name:   "foo",
								Labels: map[string]string{"env": "prod"},
							},
						}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Project{
							ObjectMeta: kapi.ObjectMeta{
								Name:   "bar",
								Labels: map[string]string{"env": "dev"},
							},
						}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	projects, err := registry.ListProjects(ctx, labels.SelectorFromSet(labels.Set{"env": "dev"}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(projects.Items) != 1 || projects.Items[0].Name != "bar" {
		t.Errorf("Unexpected projects list: %#v", projects)
	}
}

func TestEtcdGetProject(t *testing.T) {
	ctx := kapi.NewContext()
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set(makeProjectKey(ctx, "foo"), runtime.EncodeOrDie(latest.Codec, &api.Project{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	project, err := registry.GetProject(ctx, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if project.Name != "foo" {
		t.Errorf("Unexpected project: %#v", project)
	}
}

func TestEtcdGetProjectNotFound(t *testing.T) {
	ctx := kapi.NewContext()
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeProjectKey(ctx, "foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	project, err := registry.GetProject(ctx, "foo")
	if err == nil {
		t.Errorf("Unexpected non-error.")
	}
	if project != nil {
		t.Errorf("Unexpected project: %#v", project)
	}
}

func TestEtcdCreateProject(t *testing.T) {
	ctx := kapi.NewContext()
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.Data[makeProjectKey(ctx, "foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateProject(ctx, &api.Project{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get(makeProjectKey(ctx, "foo"), false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var project api.Project
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &project)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if project.Name != "foo" {
		t.Errorf("Unexpected project: %#v %s", project, resp.Node.Value)
	}
}

func TestEtcdCreateProjectAlreadyExists(t *testing.T) {
	ctx := kapi.NewContext()
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeProjectKey(ctx, "foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.Project{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateProject(ctx, &api.Project{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	})
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsAlreadyExists(err) {
		t.Errorf("Expected 'already exists' error, got %#v", err)
	}
}

func TestEtcdUpdateProject(t *testing.T) {
	ctx := kapi.NewContext()
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	err := registry.UpdateProject(ctx, &api.Project{})
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteProjectNotFound(t *testing.T) {
	ctx := kapi.NewContext()
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = tools.EtcdErrorNotFound
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteProject(ctx, "foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("Expected 'not found' error, got %#v", err)
	}
}

func TestEtcdDeleteProjectError(t *testing.T) {
	ctx := kapi.NewContext()
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = fmt.Errorf("Some error")
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteProject(ctx, "foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteProjectOK(t *testing.T) {
	ctx := kapi.NewContext()
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	key := makeProjectKey(ctx, "foo")
	err := registry.DeleteProject(ctx, "foo")
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if len(fakeClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeClient.DeletedKeys)
	} else if fakeClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeClient.DeletedKeys[0], key)
	}
}
