package originpolymorphichelpers

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsv1 "github.com/openshift/api/apps/v1"
)

func NewProtocolsForObjectFn(delegate polymorphichelpers.ProtocolsForObjectFunc) polymorphichelpers.ProtocolsForObjectFunc {
	return func(object runtime.Object) (map[string]string, error) {
		switch t := object.(type) {
		case *appsv1.DeploymentConfig:
			return getProtocols(t.Spec.Template.Spec), nil

		default:
			return delegate(object)
		}
	}
}

func getProtocols(spec corev1.PodSpec) map[string]string {
	result := make(map[string]string)
	for _, container := range spec.Containers {
		for _, port := range container.Ports {
			result[strconv.Itoa(int(port.ContainerPort))] = string(port.Protocol)
		}
	}
	return result
}
