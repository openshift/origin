package originpolymorphichelpers

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsapi "github.com/openshift/api/apps"
	deploymentcmd "github.com/openshift/origin/pkg/oc/originpolymorphichelpers/deploymentconfigs"
)

func NewStatusViewerFn(delegate polymorphichelpers.StatusViewerFunc) polymorphichelpers.StatusViewerFunc {
	return func(mapping *meta.RESTMapping) (kubectl.StatusViewer, error) {
		if appsapi.Kind("DeploymentConfig") == mapping.GroupVersionKind.GroupKind() {
			return deploymentcmd.NewDeploymentConfigStatusViewer(), nil
		}

		return delegate(mapping)
	}
}
