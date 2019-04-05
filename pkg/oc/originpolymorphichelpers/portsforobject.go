package originpolymorphichelpers

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsv1 "github.com/openshift/api/apps/v1"
)

func NewPortsForObjectFn(delegate polymorphichelpers.PortsForObjectFunc) polymorphichelpers.PortsForObjectFunc {
	return func(object runtime.Object) ([]string, error) {
		switch t := object.(type) {
		case *appsv1.DeploymentConfig:
			return getPorts(t.Spec.Template.Spec), nil

		default:
			return delegate(object)
		}
	}
}

func getPorts(spec corev1.PodSpec) []string {
	result := []string{}
	for _, container := range spec.Containers {
		for _, port := range container.Ports {
			result = append(result, strconv.Itoa(int(port.ContainerPort)))
		}
	}
	return result
}
