package originpolymorphichelpers

import (
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

func NewPortsForObjectFn(delegate polymorphichelpers.PortsForObjectFunc) polymorphichelpers.PortsForObjectFunc {
	return func(object runtime.Object) ([]string, error) {
		switch t := object.(type) {
		case *appsapi.DeploymentConfig:
			return getPorts(t.Spec.Template.Spec), nil

		default:
			return delegate(object)
		}
	}
}

func getPorts(spec api.PodSpec) []string {
	result := []string{}
	for _, container := range spec.Containers {
		for _, port := range container.Ports {
			result = append(result, strconv.Itoa(int(port.ContainerPort)))
		}
	}
	return result
}
