package originpolymorphichelpers

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	kinternalclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/printers"

	"github.com/openshift/origin/pkg/oc/lib/describe"
)

func NewDescriberFn(delegate kcmdutil.DescriberFunc) kcmdutil.DescriberFunc {
	return func(restClientGetter genericclioptions.RESTClientGetter, mapping *meta.RESTMapping) (printers.Describer, error) {

		// TODO we need to refactor the describer logic to handle misses or run serverside.
		// for now we can special case our "sometimes origin, sometimes kube" resource
		// I think it is correct for more code if this is NOT considered an origin type since
		// it wasn't an origin type pre 3.6.
		clientConfig, err := restClientGetter.ToRESTConfig()
		if err != nil {
			return nil, fmt.Errorf("unable to create client config %s: %v", mapping.GroupVersionKind.Kind, err)
		}
		kClient, err := kinternalclient.NewForConfig(clientConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to create client %s: %v", mapping.GroupVersionKind.Kind, err)
		}
		describer, ok := describe.DescriberFor(mapping.GroupVersionKind.GroupKind(), clientConfig, kClient, clientConfig.Host)
		if ok {
			return describer, nil
		}

		return delegate(restClientGetter, mapping)
	}
}
