package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/origin/pkg/service/admission/apis/restrictedendpoints"
	"github.com/openshift/origin/pkg/service/admission/apis/restrictedendpoints/v1"
)

func InstallInternal(scheme *runtime.Scheme) {
	utilruntime.Must(restrictedendpoints.Install(scheme))
	utilruntime.Must(v1.Install(scheme))
}
