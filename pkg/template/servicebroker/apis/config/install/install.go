package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	configapi "github.com/openshift/origin/pkg/template/servicebroker/apis/config"
	configapiv1 "github.com/openshift/origin/pkg/template/servicebroker/apis/config/v1"
)

// Install registers the API group and adds types to a scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(configapi.AddToScheme(scheme))
	utilruntime.Must(configapiv1.AddToScheme(scheme))
	utilruntime.Must(scheme.SetVersionPriority(configapiv1.SchemeGroupVersion))
}
