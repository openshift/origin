package originpolymorphichelpers

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	oapps "github.com/openshift/api/apps"
	deploymentcmd "github.com/openshift/origin/pkg/oc/originpolymorphichelpers/deploymentconfigs"
)

func NewHistoryViewerFn(delegate polymorphichelpers.HistoryViewerFunc) polymorphichelpers.HistoryViewerFunc {
	return func(restClientGetter genericclioptions.RESTClientGetter, mapping *meta.RESTMapping) (kubectl.HistoryViewer, error) {
		if oapps.Kind("DeploymentConfig") == mapping.GroupVersionKind.GroupKind() {
			config, err := restClientGetter.ToRESTConfig()
			if err != nil {
				return nil, err
			}
			coreClient, err := kubernetes.NewForConfig(config)
			if err != nil {
				return nil, err
			}

			return deploymentcmd.NewDeploymentConfigHistoryViewer(coreClient), nil
		}
		return delegate(restClientGetter, mapping)
	}
}
