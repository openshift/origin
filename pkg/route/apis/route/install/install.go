package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	routev1 "github.com/openshift/api/route/v1"
	routeapiv1 "github.com/openshift/origin/pkg/route/apis/route/v1"
)

func init() {
	Install(legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(routeapiv1.Install(scheme))
	utilruntime.Must(scheme.SetVersionPriority(routev1.GroupVersion))
}
