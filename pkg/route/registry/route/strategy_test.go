package route

import (
	"testing"

	"github.com/openshift/origin/pkg/route/api"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/types"
)

type testAllocator struct {
}

func (t testAllocator) AllocateRouterShard(*api.Route) (*api.RouterShard, error) {
	return &api.RouterShard{}, nil
}
func (t testAllocator) GenerateHostname(*api.Route, *api.RouterShard) string {
	return "mygeneratedhost.com"
}

func TestEmptyHostDefaulting(t *testing.T) {
	strategy := NewStrategy(testAllocator{})

	hostlessCreatedRoute := &api.Route{}
	strategy.PrepareForCreate(hostlessCreatedRoute)
	if hostlessCreatedRoute.Spec.Host != "mygeneratedhost.com" {
		t.Fatalf("Expected host to be allocated, got %s", hostlessCreatedRoute.Spec.Host)
	}

	persistedRoute := &api.Route{
		ObjectMeta: kapi.ObjectMeta{
			Namespace:       "foo",
			Name:            "myroute",
			UID:             types.UID("abc"),
			ResourceVersion: "1",
		},
		Spec: api.RouteSpec{
			Host: "myhost.com",
		},
	}
	obj, _ := kapi.Scheme.DeepCopy(persistedRoute)
	hostlessUpdatedRoute := obj.(*api.Route)
	hostlessUpdatedRoute.Spec.Host = ""
	strategy.PrepareForUpdate(hostlessUpdatedRoute, persistedRoute)
	if hostlessUpdatedRoute.Spec.Host != "myhost.com" {
		t.Fatalf("expected empty spec.host to default to existing spec.host, got %s", hostlessUpdatedRoute.Spec.Host)
	}
}
