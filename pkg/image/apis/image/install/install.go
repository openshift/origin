package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/api/image/docker10"
	"github.com/openshift/api/image/dockerpre012"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
)

func init() {
	Install(legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(docker10.AddToScheme(scheme))
	utilruntime.Must(dockerpre012.AddToScheme(scheme))
	utilruntime.Must(imageapi.AddToScheme(scheme))
	utilruntime.Must(imageapiv1.AddToScheme(scheme))
	utilruntime.Must(scheme.SetVersionPriority(imageapiv1.SchemeGroupVersion))
}
