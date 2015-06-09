package etcd

import (
	"fmt"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools/etcdtest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/coreos/go-etcd/etcd"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/route/api"
	_ "github.com/openshift/origin/pkg/route/api/v1beta3"
)

// This copy and paste is not pure ignorance.  This is that we can be sure that the key is getting made as we
// expect it to. If someone changes the location of these resources by say moving all the resources to
// "/origin/resources" (which is a really good idea), then they've made a breaking change and something should
// fail to let them know they've change some significant change and that other dependent pieces may break.
func makeTestRouteListKey(namespace string) string {
	if len(namespace) != 0 {
		return "/routes/" + namespace
	}
	return "/routes"
}
func makeTestRouteKey(namespace, id string) string {
	return "/routes/" + namespace + "/" + id
}
func makeTestDefaultRouteKey(id string) string {
	return makeTestRouteKey(kapi.NamespaceDefault, id)
}
func makeTestDefaultRouteListKey() string {
	return makeTestRouteListKey(kapi.NamespaceDefault)
}

func NewTestEtcd(client tools.EtcdClient) *Etcd {
	return New(tools.NewEtcdHelper(client, latest.Codec, etcdtest.PathPrefix()))
}

func TestEtcdListEmptyRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultRouteListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	routes, err := registry.ListRoutes(kapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(routes.Items) != 0 {
		t.Errorf("Unexpected routes list: %#v", routes)
	}
}

func TestEtcdListErrorRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultRouteListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: fmt.Errorf("some error"),
	}
	registry := NewTestEtcd(fakeClient)
	routes, err := registry.ListRoutes(kapi.NewDefaultContext(), labels.Everything())
	if err == nil {
		t.Error("unexpected nil error")
	}

	if routes != nil {
		t.Errorf("Unexpected non-nil routes: %#v", routes)
	}
}

func TestEtcdListEverythingRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultRouteListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Route{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Route{ObjectMeta: kapi.ObjectMeta{Name: "bar"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	routes, err := registry.ListRoutes(kapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(routes.Items) != 2 || routes.Items[0].Name != "foo" || routes.Items[1].Name != "bar" {
		t.Errorf("Unexpected routes list: %#v", routes)
	}
}

func TestEtcdListFilteredRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultRouteListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Route{
							ObjectMeta: kapi.ObjectMeta{
								Name:   "foo",
								Labels: map[string]string{"env": "prod"},
							},
						}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Route{
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
	routes, err := registry.ListRoutes(kapi.NewDefaultContext(), labels.SelectorFromSet(labels.Set{"env": "dev"}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(routes.Items) != 1 || routes.Items[0].Name != "bar" {
		t.Errorf("Unexpected routes list: %#v", routes)
	}
}

func TestEtcdGetRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set(makeTestDefaultRouteKey("foo"), runtime.EncodeOrDie(latest.Codec, &api.Route{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	route, err := registry.GetRoute(kapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if route.Name != "foo" {
		t.Errorf("Unexpected route: %#v", route)
	}
}

func TestEtcdGetNotFoundRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultRouteKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	route, err := registry.GetRoute(kapi.NewDefaultContext(), "foo")
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
	fakeClient.Data[makeTestDefaultRouteKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateRoute(kapi.NewDefaultContext(), &api.Route{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get(makeTestDefaultRouteKey("foo"), false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var route api.Route
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &route)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if route.Name != "foo" {
		t.Errorf("Unexpected route: %#v %s", route, resp.Node.Value)
	}
}

func TestEtcdCreateAlreadyExistsRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultRouteKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.Route{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateRoute(kapi.NewDefaultContext(), &api.Route{
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

func TestEtcdUpdateOkRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	err := registry.UpdateRoute(kapi.NewDefaultContext(), &api.Route{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	})
	if err != nil {
		t.Errorf("Unexpected error %#v", err)
	}
}

func TestEtcdDeleteNotFoundRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = tools.EtcdErrorNotFound
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteRoute(kapi.NewDefaultContext(), "foo")
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
	err := registry.DeleteRoute(kapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteOkRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	key := makeTestDefaultRouteListKey() + "/foo"
	fakeClient.Set(key, runtime.EncodeOrDie(latest.Codec, &api.Route{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	err := registry.DeleteRoute(kapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if len(fakeClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeClient.DeletedKeys)
	} else if fakeClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeClient.DeletedKeys[0], key)
	}
}

func TestEtcdCreateRouteFailsWithoutNamespace(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateRoute(kapi.NewContext(), &api.Route{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	})

	if err == nil {
		t.Errorf("expected error that namespace was missing from context")
	}
}

func TestEtcdListRoutesInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kapi.WithNamespace(kapi.NewContext(), "alfa")
	namespaceBravo := kapi.WithNamespace(kapi.NewContext(), "bravo")
	fakeClient.Data["/routes/alfa"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Route{ObjectMeta: kapi.ObjectMeta{Name: "foo1"}}),
					},
				},
			},
		},
		E: nil,
	}
	fakeClient.Data["/routes/bravo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Route{ObjectMeta: kapi.ObjectMeta{Name: "foo2"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Route{ObjectMeta: kapi.ObjectMeta{Name: "bar2"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)

	routesAlfa, err := registry.ListRoutes(namespaceAlfa, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(routesAlfa.Items) != 1 || routesAlfa.Items[0].Name != "foo1" {
		t.Errorf("Unexpected builds list: %#v", routesAlfa)
	}

	routesBravo, err := registry.ListRoutes(namespaceBravo, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(routesBravo.Items) != 2 || routesBravo.Items[0].Name != "foo2" || routesBravo.Items[1].Name != "bar2" {
		t.Errorf("Unexpected builds list: %#v", routesBravo)
	}
}

func TestEtcdGetRouteInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kapi.WithNamespace(kapi.NewContext(), "alfa")
	namespaceBravo := kapi.WithNamespace(kapi.NewContext(), "bravo")
	fakeClient.Set("/routes/alfa/foo", runtime.EncodeOrDie(latest.Codec, &api.Route{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	fakeClient.Set("/routes/bravo/foo", runtime.EncodeOrDie(latest.Codec, &api.Route{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)

	alfaFoo, err := registry.GetRoute(namespaceAlfa, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if alfaFoo == nil || alfaFoo.Name != "foo" {
		t.Errorf("Unexpected deployment: %#v", alfaFoo)
	}

	bravoFoo, err := registry.GetRoute(namespaceBravo, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if bravoFoo == nil || bravoFoo.Name != "foo" {
		t.Errorf("Unexpected deployment: %#v", bravoFoo)
	}
}

func TestEtcdWatchRoutes(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)

	watching, err := registry.WatchRoutes(kapi.NewDefaultContext(), labels.Everything(), fields.Everything(), "1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fakeClient.WaitForWatchCompletion()

	route := &api.Route{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	}

	routeBytes, _ := latest.Codec.Encode(route)

	fakeClient.WatchResponse <- &etcd.Response{
		Action: "set",
		Node: &etcd.Node{
			Value: string(routeBytes),
		},
	}

	event := <-watching.ResultChan()

	if event.Type != watch.Added {
		t.Errorf("Expected add but got %s", event.Type)
	}

	if !reflect.DeepEqual(route, event.Object) {
		t.Errorf("Expected %v, got %v", route, event.Object)
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
