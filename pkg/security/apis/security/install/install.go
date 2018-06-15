package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/origin/pkg/api/legacy"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securityapiv1 "github.com/openshift/origin/pkg/security/apis/security/v1"
)

func init() {
	legacy.InstallLegacySecurity(legacyscheme.Scheme)
	Install(legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(securityapi.AddToScheme(scheme))
	utilruntime.Must(securityapiv1.AddToScheme(scheme))
	utilruntime.Must(scheme.SetVersionPriority(securityapiv1.SchemeGroupVersion))
}
