package etcd

import (
	"fmt"
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/coreos/go-etcd/etcd"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/route/api"
	_ "github.com/openshift/origin/pkg/route/api/v1beta1"
)

func NewTestEtcd(client tools.EtcdClient) *Etcd {
	return New(tools.EtcdHelper{client, latest.Codec, latest.ResourceVersioner})
}

func TestEtcdListEmptyRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/routes"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	routes, err := registry.ListRoutes(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(routes.Items) != 0 {
		t.Errorf("Unexpected routes list: %#v", routes)
	}
}

func TestEtcdListErrorRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/routes"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: fmt.Errorf("some error"),
	}
	registry := NewTestEtcd(fakeClient)
	routes, err := registry.ListRoutes(labels.Everything())
	if err == nil {
		t.Error("unexpected nil error")
	}

	if routes != nil {
		t.Errorf("Unexpected non-nil routes: %#v", routes)
	}
}

func TestEtcdListEverythingRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/routes"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Route{JSONBase: kubeapi.JSONBase{ID: "foo"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Route{JSONBase: kubeapi.JSONBase{ID: "bar"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	routes, err := registry.ListRoutes(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(routes.Items) != 2 || routes.Items[0].ID != "foo" || routes.Items[1].ID != "bar" {
		t.Errorf("Unexpected routes list: %#v", routes)
	}
}

func TestEtcdListFilteredRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/routes"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Route{
							JSONBase: kubeapi.JSONBase{ID: "foo"},
							Labels:   map[string]string{"env": "prod"},
						}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Route{
							JSONBase: kubeapi.JSONBase{ID: "bar"},
							Labels:   map[string]string{"env": "dev"},
						}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	routes, err := registry.ListRoutes(labels.SelectorFromSet(labels.Set{"env": "dev"}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(routes.Items) != 1 || routes.Items[0].ID != "bar" {
		t.Errorf("Unexpected routes list: %#v", routes)
	}
}

func TestEtcdGetRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set("/routes/foo", runtime.EncodeOrDie(latest.Codec, &api.Route{JSONBase: kubeapi.JSONBase{ID: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	route, err := registry.GetRoute("foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if route.ID != "foo" {
		t.Errorf("Unexpected route: %#v", route)
	}
}

func TestEtcdGetNotFoundRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data["/routes/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	route, err := registry.GetRoute("foo")
	if err == nil {
		t.Errorf("Unexpected non-error.")
	}
	if route != nil {
		t.Errorf("Unexpected route: %#v", route)
	}
}

func TestEtcdCreateRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.Data["/routes/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateRoute(&api.Route{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get("/routes/foo", false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var route api.Route
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &route)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if route.ID != "foo" {
		t.Errorf("Unexpected route: %#v %s", route, resp.Node.Value)
	}
}

func TestEtcdCreateAlreadyExistsRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data["/routes/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.Route{JSONBase: kubeapi.JSONBase{ID: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateRoute(&api.Route{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
		},
	})
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsAlreadyExists(err) {
		t.Errorf("Expected 'already exists' error, got %#v", err)
	}
}

func TestEtcdUpdateOkRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	err := registry.UpdateRoute(&api.Route{})
	if err != nil {
		t.Error("Unexpected error")
	}
}

func TestEtcdDeleteNotFoundRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = tools.EtcdErrorNotFound
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteRoute("foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("Expected 'not found' error, got %#v", err)
	}
}

func TestEtcdDeleteErrorRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = fmt.Errorf("Some error")
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteRoute("foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteOkRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	key := "/routes/foo"
	err := registry.DeleteRoute("foo")
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if len(fakeClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeClient.DeletedKeys)
	} else if fakeClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeClient.DeletedKeys[0], key)
	}
}
