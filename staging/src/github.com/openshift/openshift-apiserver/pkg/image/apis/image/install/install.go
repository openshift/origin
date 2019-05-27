package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	imagev1 "github.com/openshift/api/image/v1"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
)

func init() {
	Install(legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(imageapiv1.Install(scheme))
	utilruntime.Must(scheme.SetVersionPriority(imagev1.SchemeGroupVersion))
}
