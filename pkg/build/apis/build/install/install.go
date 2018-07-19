package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	buildv1 "github.com/openshift/api/build/v1"
	buildapiv1 "github.com/openshift/origin/pkg/build/apis/build/v1"
)

func init() {
	Install(legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(buildapiv1.Install(scheme))
	utilruntime.Must(scheme.SetVersionPriority(buildv1.GroupVersion))
}
