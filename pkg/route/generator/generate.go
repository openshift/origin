package generator

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/route/api"
)

// RouteGenerator implements the kubectl.Generator interface for routes
type RouteGenerator struct{}

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
func (RouteGenerator) Generate(params map[string]string) (runtime.Object, error) {
	var (
		labels map[string]string
		err    error
	)

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
		Host:        params["hostname"],
		ServiceName: params["default-name"],
	}, nil
}

// Useful pattern for validating that RouteGenerator implements
// the Generator interface
var _ kubectl.Generator = RouteGenerator{}
