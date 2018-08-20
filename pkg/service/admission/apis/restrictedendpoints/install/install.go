package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/origin/pkg/service/admission/apis/restrictedendpoints"
	"github.com/openshift/origin/pkg/service/admission/apis/restrictedendpoints/v1"
)

func InstallLegacyInternal(scheme *runtime.Scheme) {
	utilruntime.Must(restrictedendpoints.InstallLegacy(scheme))
	utilruntime.Must(v1.InstallLegacy(scheme))
}
