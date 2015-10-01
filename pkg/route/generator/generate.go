package generator

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/route/api"
)

// RouteGenerator generates routes from a given set of parameters
type RouteGenerator struct{}

// RouteGenerator implements the kubectl.Generator interface for routes
var _ kubectl.Generator = RouteGenerator{}

// ParamNames returns the parameters required for generating a route
func (RouteGenerator) ParamNames() []kubectl.GeneratorParam {
	return []kubectl.GeneratorParam{
		{"labels", false},
		{"default-name", true},
		{"name", false},
		{"hostname", false},
	}
}

// Generate accepts a set of parameters and maps them into a new route
func (RouteGenerator) Generate(genericParams map[string]interface{}) (runtime.Object, error) {
	var (
		labels map[string]string
		err    error
	)

	params := map[string]string{}
	for key, value := range genericParams {
		strVal, isString := value.(string)
		if !isString {
			return nil, fmt.Errorf("expected string, saw %v for '%s'", value, key)
		}
		params[key] = strVal
	}

	labelString, found := params["labels"]
	if found && len(labelString) > 0 {
		labels, err = kubectl.ParseLabels(labelString)
		if err != nil {
			return nil, err
		}
	}

	name, found := params["name"]
	if !found || len(name) == 0 {
		name, found = params["default-name"]
		if !found || len(name) == 0 {
			return nil, fmt.Errorf("'name' is a required parameter.")
		}
	}

	return &api.Route{
		ObjectMeta: kapi.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: api.RouteSpec{
			Host: params["hostname"],
			To: kapi.ObjectReference{
				Name: params["default-name"],
			},
		},
	}, nil
}
