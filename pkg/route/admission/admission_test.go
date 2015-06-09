package admission

import (
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// NewTestPlugin creates a new plugin with an injected store for testing.
func NewTestPlugin(store cache.Store) admission.Interface {
	return &routeUniqueness{
		Handler:    routeAdmissionHandler(),
		routeStore: store,
	}
}

func TestHandler(t *testing.T) {
	plugin := NewTestPlugin(nil)
	if plugin.Handles(admission.Connect) {
		t.Errorf("plugin should not handle connect")
	}
	if plugin.Handles(admission.Delete) {
		t.Errorf("plugin should not handle delete")
	}
	if !plugin.Handles(admission.Create) {
		t.Errorf("plugin should handle create")
	}
	if !plugin.Handles(admission.Update) {
		t.Errorf("plugin should handle update")
	}
}

func TestAdmission(t *testing.T) {
	newValidRoute := func(routeType string) *routeapi.Route {
		route := &routeapi.Route{
			ObjectMeta: api.ObjectMeta{
				Namespace: "namespace",
				Name:      "name",
			},
			Host: "www.example.com",
		}
		if strings.Contains(routeType, "path") {
			route.Path = "/foo"
		}

		switch routeType {
		case "secure_edge", "secure_edge_path":
			route.TLS = &routeapi.TLSConfig{
				Termination: routeapi.TLSTerminationEdge,
			}
		case "secure_reencrypt", "secure_reencrypt_path":
			route.TLS = &routeapi.TLSConfig{
				Termination: routeapi.TLSTerminationReencrypt,
			}

		case "secure_passthrough", "secure_passthrough_path":
			route.TLS = &routeapi.TLSConfig{
				Termination: routeapi.TLSTerminationPassthrough,
			}
		}
		return route
	}

	testCases := map[string]struct {
		attributes   admission.Attributes
		shouldPass   bool
		storeObjects []runtime.Object
	}{
		"bad object": {
			attributes: admission.NewAttributesRecord(nil, "Route", "", "routes", admission.Create, nil),
			shouldPass: false,
		},
		"add unsecure, no match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("unsecure"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   true,
			storeObjects: []runtime.Object{newValidRoute("unsecure_path"), newValidRoute("secure_edge"), newValidRoute("secure_edge_path")},
		},
		"add unsecure path, no match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("unsecure_path"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   true,
			storeObjects: []runtime.Object{newValidRoute("unsecure"), newValidRoute("secure_edge"), newValidRoute("secure_edge_path")},
		},
		"add unsecure, match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("unsecure"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   false,
			storeObjects: []runtime.Object{newValidRoute("unsecure")},
		},
		"add unsecure path, match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("unsecure_path"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   false,
			storeObjects: []runtime.Object{newValidRoute("unsecure_path")},
		},
		"add edge, no match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("secure_edge"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   true,
			storeObjects: []runtime.Object{newValidRoute("unsecure_path"), newValidRoute("unsecure"), newValidRoute("secure_edge_path")},
		},
		"add edge path, no match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("secure_edge_path"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   true,
			storeObjects: []runtime.Object{newValidRoute("unsecure_path"), newValidRoute("unsecure"), newValidRoute("secure_edge")},
		},
		"add edge, match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("secure_edge"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   false,
			storeObjects: []runtime.Object{newValidRoute("secure_edge")},
		},
		"add edge path, match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("secure_edge_path"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   false,
			storeObjects: []runtime.Object{newValidRoute("secure_edge_path")},
		},
		"add reencrypt, no match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("secure_reencrypt"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   true,
			storeObjects: []runtime.Object{newValidRoute("unsecure_path"), newValidRoute("unsecure"), newValidRoute("secure_reencrypt_path")},
		},
		"add reencrypt path, no match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("secure_reencrypt_path"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   true,
			storeObjects: []runtime.Object{newValidRoute("unsecure_path"), newValidRoute("unsecure"), newValidRoute("secure_reencrypt")},
		},
		"add reencrypt, match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("secure_reencrypt"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   false,
			storeObjects: []runtime.Object{newValidRoute("secure_reencrypt")},
		},
		"add reencrypt path, match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("secure_reencrypt_path"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   false,
			storeObjects: []runtime.Object{newValidRoute("secure_reencrypt_path")},
		},
		"add passthrough, no match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("secure_passthrough"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   true,
			storeObjects: []runtime.Object{newValidRoute("unsecure_path"), newValidRoute("unsecure"), newValidRoute("secure_passthrough_path")},
		},
		"add passthrough path, no match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("secure_passthrough_path"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   true,
			storeObjects: []runtime.Object{newValidRoute("unsecure_path"), newValidRoute("unsecure"), newValidRoute("secure_passthrough")},
		},
		"add passthrough, match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("secure_passthrough"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   false,
			storeObjects: []runtime.Object{newValidRoute("secure_passthrough")},
		},
		"add passthrough path, match": {
			attributes:   admission.NewAttributesRecord(newValidRoute("secure_passthrough_path"), "Route", "", "routes", admission.Create, nil),
			shouldPass:   false,
			storeObjects: []runtime.Object{newValidRoute("secure_passthrough_path")},
		},
		"update, namespace doesn't match": {
			attributes: admission.NewAttributesRecord(&routeapi.Route{
				ObjectMeta: api.ObjectMeta{
					Namespace: "namespace2",
					Name:      "name",
				},
				Host: "www.example.com",
			}, "Route", "", "routes", admission.Update, nil),
			shouldPass:   false,
			storeObjects: []runtime.Object{newValidRoute("unsecure")},
		},
		"update, name doesn't match": {
			attributes: admission.NewAttributesRecord(&routeapi.Route{
				ObjectMeta: api.ObjectMeta{
					Namespace: "namespace",
					Name:      "name2",
				},
				Host: "www.example.com",
			}, "Route", "", "routes", admission.Update, nil),
			shouldPass:   false,
			storeObjects: []runtime.Object{newValidRoute("unsecure")},
		},
		"update, name/namespace matches": {
			attributes:   admission.NewAttributesRecord(newValidRoute("unsecure"), "Route", "", "routes", admission.Update, nil),
			shouldPass:   true,
			storeObjects: []runtime.Object{newValidRoute("unsecure")},
		},
	}

	for k, v := range testCases {
		store := cache.NewStore(StoreKeyFunc)
		for _, obj := range v.storeObjects {
			store.Add(obj)
		}
		plugin := NewTestPlugin(store)
		err := plugin.Admit(v.attributes)
		if v.shouldPass && err != nil {
			t.Errorf("unexpected error in %s, should have passed but received %v", k, err)
		}
		if !v.shouldPass && err == nil {
			t.Errorf("%s expected an error but received none", k)
		}
	}
}
