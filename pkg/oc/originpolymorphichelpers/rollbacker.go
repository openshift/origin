package originpolymorphichelpers

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	oapps "github.com/openshift/api/apps"
	appsclient "github.com/openshift/client-go/apps/clientset/versioned"
	deploymentcmd "github.com/openshift/origin/pkg/oc/originpolymorphichelpers/deploymentconfigs"
)

func NewRollbackerFn(delegate polymorphichelpers.RollbackerFunc) polymorphichelpers.RollbackerFunc {
	return func(restClientGetter genericclioptions.RESTClientGetter, mapping *meta.RESTMapping) (kubectl.Rollbacker, error) {
		if oapps.Kind("DeploymentConfig") == mapping.GroupVersionKind.GroupKind() {
			config, err := restClientGetter.ToRESTConfig()
			if err != nil {
				return nil, err
			}
			appsClient, err := appsclient.NewForConfig(config)
			if err != nil {
				return nil, err
			}
			return deploymentcmd.NewDeploymentConfigRollbacker(appsClient), nil
		}
		return delegate(restClientGetter, mapping)
	}
}
