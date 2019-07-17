package originpolymorphichelpers

import (
	"k8s.io/kubernetes/pkg/kubectl/generate"
	"k8s.io/kubernetes/pkg/kubectl/generate/versioned"

	deploymentcmd "github.com/openshift/oc/pkg/helpers/originpolymorphichelpers/deploymentconfigs"
	routegen "github.com/openshift/oc/pkg/helpers/route/generator"
)

func defaultGenerators(cmdName string) map[string]generate.Generator {
	generators := map[string]map[string]generate.Generator{}
	generators["run"] = map[string]generate.Generator{
		"deploymentconfig/v1": deploymentcmd.BasicDeploymentConfigController{},
		"run-controller/v1":   versioned.BasicReplicationController{}, // legacy alias for run/v1
	}
	generators["expose"] = map[string]generate.Generator{
		"route/v1": routegen.RouteGenerator{},
	}

	return generators[cmdName]
}

func NewGeneratorsFn(delegate generate.GeneratorFunc) generate.GeneratorFunc {
	return func(cmdName string) map[string]generate.Generator {
		originGenerators := defaultGenerators(cmdName)
		kubeGenerators := delegate(cmdName)

		ret := map[string]generate.Generator{}
		for k, v := range kubeGenerators {
			ret[k] = v
		}
		for k, v := range originGenerators {
			ret[k] = v
		}
		return ret
	}
}
