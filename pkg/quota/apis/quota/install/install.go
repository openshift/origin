package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	quotav1 "github.com/openshift/api/quota/v1"
	quotaapiv1 "github.com/openshift/origin/pkg/quota/apis/quota/v1"
)

func init() {
	Install(legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(quotaapiv1.Install(scheme))
	utilruntime.Must(scheme.SetVersionPriority(quotav1.GroupVersion))
}
