package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/origin/pkg/build/controller/build/apis/defaults"
	"github.com/openshift/origin/pkg/build/controller/build/apis/defaults/v1"
)

func InstallLegacyInternal(scheme *runtime.Scheme) {
	utilruntime.Must(defaults.InstallLegacy(scheme))
	utilruntime.Must(v1.InstallLegacy(scheme))
}
