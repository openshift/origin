package allocation

import (
	"fmt"
	"testing"

	routeapi "github.com/openshift/origin/pkg/route/api"
	kapi "k8s.io/kubernetes/pkg/api"
)

type TestAllocationPlugin struct {
	Name string
}

func (p *TestAllocationPlugin) Allocate(route *routeapi.Route) (*routeapi.RouterShard, error) {

	return &routeapi.RouterShard{ShardName: "test", DNSSuffix: "openshift.test"}, nil
}

func (p *TestAllocationPlugin) GenerateHostname(route *routeapi.Route, shard *routeapi.RouterShard) string {
	if len(route.Spec.To.Name) > 0 && len(route.Namespace) > 0 {
		return fmt.Sprintf("%s-%s.%s", route.Spec.To.Name, route.Namespace, shard.DNSSuffix)
	}

	return "test-test-test.openshift.test"
}

func TestRouteAllocationController(t *testing.T) {
	tests := []struct {
		name  string
		route *routeapi.Route
	}{
		{
			name: "No Name",
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
				},
				Spec: routeapi.RouteSpec{
					To: kapi.ObjectReference{
						Name: "service",
					},
				},
			},
		},
		{
			name: "No namespace",
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name: "name",
				},
				Spec: routeapi.RouteSpec{
					To: kapi.ObjectReference{
						Name: "nonamespace",
					},
				},
			},
		},
		{
			name: "No service name",
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
			},
		},
		{
			name: "Valid route",
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.org",
					To: kapi.ObjectReference{
						Name: "serviceName",
					},
				},
			},
		},
	}

	plugin := &TestAllocationPlugin{Name: "test allocation plugin"}
	fac := &RouteAllocationControllerFactory{nil, nil}
	allocator := fac.Create(plugin)
	for _, tc := range tests {
		shard, err := allocator.AllocateRouterShard(tc.route)
		if err != nil {
			t.Errorf("Test case %s got an error %s", tc.name, err)
			continue
		}
		name := allocator.GenerateHostname(tc.route, shard)
		if len(name) <= 0 {
			t.Errorf("Test case %s got %d length name", tc.name, len(name))
		}
	}
}
