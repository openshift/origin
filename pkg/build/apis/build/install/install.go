package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/origin/pkg/api/legacy"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildapiv1 "github.com/openshift/origin/pkg/build/apis/build/v1"
)

func init() {
	legacy.InstallLegacyBuild(legacyscheme.Scheme)
	Install(legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(buildapi.AddToScheme(scheme))
	utilruntime.Must(buildapiv1.AddToScheme(scheme))
	utilruntime.Must(scheme.SetVersionPriority(buildapiv1.SchemeGroupVersion))
}
