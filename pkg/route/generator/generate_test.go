package generator

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

func TestGenerateRoute(t *testing.T) {
	generator := RouteGenerator{}

	tests := []struct {
		params   map[string]interface{}
		expected routeapi.Route
	}{
		{
			params: map[string]interface{}{
				"labels":       "foo=bar",
				"name":         "test",
				"default-name": "someservice",
				"port":         "80",
				"ports":        "80,443",
				"target-port":  "svcportname",
				"hostname":     "www.example.com",
			},
			expected: routeapi.Route{
				ObjectMeta: api.ObjectMeta{
					Name: "test",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					To: api.ObjectReference{
						Name: "someservice",
					},
					Port: &routeapi.RoutePort{
						TargetPort: util.IntOrString{
							Kind:   util.IntstrString,
							StrVal: "svcportname",
						},
					},
				},
			},
		},
		{
			params: map[string]interface{}{
				"labels":       "foo=bar",
				"name":         "test",
				"default-name": "someservice",
				"port":         "80",
				"ports":        "80,443",
				"hostname":     "www.example.com",
			},
			expected: routeapi.Route{
				ObjectMeta: api.ObjectMeta{
					Name: "test",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					To: api.ObjectReference{
						Name: "someservice",
					},
				},
			},
		},
	}
	for _, test := range tests {
		obj, err := generator.Generate(test.params)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(obj, &test.expected) {
			t.Errorf("expected:\n%#v\ngot\n%#v\n", &test.expected, obj)
		}
	}
}
