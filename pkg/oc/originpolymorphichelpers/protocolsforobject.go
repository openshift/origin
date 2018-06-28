package originpolymorphichelpers

import (
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

func getProtocols(spec api.PodSpec) map[string]string {
	result := make(map[string]string)
	for _, container := range spec.Containers {
		for _, port := range container.Ports {
			result[strconv.Itoa(int(port.ContainerPort))] = string(port.Protocol)
		}
	}
	return result
}

func NewProtocolsForObjectFn(delegate polymorphichelpers.ProtocolsForObjectFunc) polymorphichelpers.ProtocolsForObjectFunc {
	return func(object runtime.Object) (map[string]string, error) {
		switch t := object.(type) {
		case *appsapi.DeploymentConfig:
			return getProtocols(t.Spec.Template.Spec), nil

		default:
			return delegate(object)
		}
	}
}
