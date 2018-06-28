package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	sdnapi "github.com/openshift/origin/pkg/network/apis/network"
	sdnapiv1 "github.com/openshift/origin/pkg/network/apis/network/v1"
)

func init() {
	Install(legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(sdnapi.AddToScheme(scheme))
	utilruntime.Must(sdnapiv1.AddToScheme(scheme))
	utilruntime.Must(scheme.SetVersionPriority(sdnapiv1.SchemeGroupVersion))
}
