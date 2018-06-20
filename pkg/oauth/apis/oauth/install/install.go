package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthapiv1 "github.com/openshift/origin/pkg/oauth/apis/oauth/v1"
)

func init() {
	Install(legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(oauthapi.AddToScheme(scheme))
	utilruntime.Must(oauthapiv1.AddToScheme(scheme))
	utilruntime.Must(scheme.SetVersionPriority(oauthapiv1.SchemeGroupVersion))
}
