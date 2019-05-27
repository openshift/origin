package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	templatev1 "github.com/openshift/api/template/v1"
	templateapiv1 "github.com/openshift/origin/pkg/template/apis/template/v1"
)

func init() {
	Install(legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(templateapiv1.Install(scheme))
	utilruntime.Must(scheme.SetVersionPriority(templatev1.GroupVersion))
}
