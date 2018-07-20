package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	networkv1 "github.com/openshift/api/network/v1"
	sdnapiv1 "github.com/openshift/origin/pkg/network/apis/network/v1"
)

func init() {
	Install(legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(sdnapiv1.Install(scheme))
	utilruntime.Must(scheme.SetVersionPriority(networkv1.GroupVersion))
}
