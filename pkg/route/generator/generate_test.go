package generator

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
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
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					To: routeapi.RouteTargetReference{
						Name: "someservice",
					},
					Port: &routeapi.RoutePort{
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 80,
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
				"port":         "web",
				"ports":        "80,443",
				"hostname":     "www.example.com",
			},
			expected: routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					Port: &routeapi.RoutePort{
						TargetPort: intstr.IntOrString{
							Type:   intstr.String,
							StrVal: "web",
						},
					},
					To: routeapi.RouteTargetReference{
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
