package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/origin/pkg/scheduler/admission/apis/podnodeconstraints"
	"github.com/openshift/origin/pkg/scheduler/admission/apis/podnodeconstraints/v1"
)

func InstallLegacyInternal(scheme *runtime.Scheme) {
	utilruntime.Must(podnodeconstraints.InstallLegacy(scheme))
	utilruntime.Must(v1.InstallLegacy(scheme))
}
