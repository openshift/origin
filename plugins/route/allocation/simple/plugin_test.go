package simple

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/route/api"
	rac "github.com/openshift/origin/pkg/route/controller/allocation"
)

func TestNewSimpleAllocationPlugin(t *testing.T) {
	tests := []struct {
		Name             string
		ErrorExpectation bool
	}{
		{
			Name:             "www.example.org",
			ErrorExpectation: false,
		},
		{
			Name:             "www^acme^org",
			ErrorExpectation: true,
		},
		{
			Name:             "bad wolf.whoswho",
			ErrorExpectation: true,
		},
		{
			Name:             "tardis#1.watch",
			ErrorExpectation: true,
		},
		{
			Name:             "こんにちはopenshift.com",
			ErrorExpectation: true,
		},
		{
			Name:             "yo!yo!@#$%%$%^&*(0){[]}:;',<>?/1.test",
			ErrorExpectation: true,
		},
		{
			Name:             "",
			ErrorExpectation: false,
		},
	}

	for _, tc := range tests {
		sap, err := NewSimpleAllocationPlugin(tc.Name)
		if err != nil && !tc.ErrorExpectation {
			t.Errorf("Test case for %s got an error where none was expected", tc.Name)
		}
		if len(tc.Name) > 0 {
			continue
		}
		dap := &SimpleAllocationPlugin{DNSSuffix: defaultDNSSuffix}
		if sap.DNSSuffix != dap.DNSSuffix {
			t.Errorf("Expected function to use defaultDNSSuffix for empty name argument.")
		}
	}
}

func TestSimpleAllocationPlugin(t *testing.T) {
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
				ServiceName: "myservice",
			},
		},
		{
			name: "No host",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				ServiceName: "myservice",
			},
		},
	}

	plugin, err := NewSimpleAllocationPlugin("www.example.org")
	if err != nil {
		t.Errorf("Error creating SimpleAllocationPlugin got %s", err)
		return
	}

	for _, tc := range tests {
		shard, _ := plugin.Allocate(tc.route)
		name := plugin.GenerateHostname(tc.route, shard)
		if len(name) <= 0 {
			t.Errorf("Test case %s got %d length name.", tc.name, len(name))
		}
		if !util.IsDNS1123Subdomain(name) {
			t.Errorf("Test case %s got %s - invalid DNS name.", tc.name, name)
		}
	}
}

func TestSimpleAllocationPluginViaController(t *testing.T) {
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
				ServiceName: "s3",
			},
		},
	}

	plugin, _ := NewSimpleAllocationPlugin("www.example.org")
	fac := &rac.RouteAllocationControllerFactory{nil, nil}
	sac := fac.Create(plugin)

	for _, tc := range tests {
		shard, err := sac.AllocateRouterShard(tc.route)
		if err != nil {
			t.Errorf("Test case %s got an error %s", tc.name, err)
		}
		name := sac.GenerateHostname(tc.route, shard)
		if len(name) <= 0 {
			t.Errorf("Test case %s got %d length name", tc.name, len(name))
		}
		if !util.IsDNS1123Subdomain(name) {
			t.Errorf("Test case %s got %s - invalid DNS name.", tc.name, name)
		}
	}
}
