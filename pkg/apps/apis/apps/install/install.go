package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsapiv1 "github.com/openshift/origin/pkg/apps/apis/apps/v1"
)

func init() {
	Install(legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(appsapi.AddToScheme(scheme))
	utilruntime.Must(appsapiv1.AddToScheme(scheme))
	utilruntime.Must(scheme.SetVersionPriority(appsapiv1.SchemeGroupVersion))
}
