package originpolymorphichelpers

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/kubectl/describe"

	odescribe "github.com/openshift/oc/pkg/helpers/describe"
)

func NewDescriberFn(delegate describe.DescriberFunc) describe.DescriberFunc {
	return func(restClientGetter genericclioptions.RESTClientGetter, mapping *meta.RESTMapping) (describe.Describer, error) {

		// TODO we need to refactor the describer logic to handle misses or run serverside.
		// for now we can special case our "sometimes origin, sometimes kube" resource
		// I think it is correct for more code if this is NOT considered an origin type since
		// it wasn't an origin type pre 3.6.
		clientConfig, err := restClientGetter.ToRESTConfig()
		if err != nil {
			return nil, fmt.Errorf("unable to create client config %s: %v", mapping.GroupVersionKind.Kind, err)
		}
		kubeClient, err := kubernetes.NewForConfig(clientConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to create client %s: %v", mapping.GroupVersionKind.Kind, err)
		}
		describer, ok := odescribe.DescriberFor(mapping.GroupVersionKind.GroupKind(), clientConfig, kubeClient, clientConfig.Host)
		if ok {
			return describer, nil
		}

		return delegate(restClientGetter, mapping)
	}
}
