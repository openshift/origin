package originpolymorphichelpers

import (
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	deploymentcmd "github.com/openshift/origin/pkg/oc/originpolymorphichelpers/deploymentconfigs"
	routegen "github.com/openshift/origin/pkg/route/generator"
)

func defaultGenerators(cmdName string) map[string]kubectl.Generator {
	generators := map[string]map[string]kubectl.Generator{}
	generators["run"] = map[string]kubectl.Generator{
		"deploymentconfig/v1": deploymentcmd.BasicDeploymentConfigController{},
		"run-controller/v1":   kubectl.BasicReplicationController{}, // legacy alias for run/v1
	}
	generators["expose"] = map[string]kubectl.Generator{
		"route/v1": routegen.RouteGenerator{},
	}

	return generators[cmdName]
}

func NewGeneratorsFn(delegate kcmdutil.GeneratorFunc) kcmdutil.GeneratorFunc {
	return func(cmdName string) map[string]kubectl.Generator {
		originGenerators := defaultGenerators(cmdName)
		kubeGenerators := delegate(cmdName)

		ret := map[string]kubectl.Generator{}
		for k, v := range kubeGenerators {
			ret[k] = v
		}
		for k, v := range originGenerators {
			ret[k] = v
		}
		return ret
	}
}
