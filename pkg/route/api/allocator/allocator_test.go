package allocator

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/route/api"
)

func TestAllocateRoute(t *testing.T) {
	tests := []struct {
		name  string
		route *api.Route
	}{
		{
			name: "No Name",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
				},
				ServiceName: "service",
			},
		},
		{
			name: "No namespace",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name: "name",
				},
				ServiceName: "nonamespace",
			},
		},
		{
			name: "No service name",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
			},
		},
		{
			name: "Valid route",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Host:        "www.example.com",
				ServiceName: "serviceName",
			},
		},
	}

	for _, tc := range tests {
		shard := Allocate(tc.route)
		name := Generate(tc.route, shard)

		if len(name) <= 0 {
			t.Errorf("Test case %s got %d length name.", tc.name, len(name))
		}
	}
}
