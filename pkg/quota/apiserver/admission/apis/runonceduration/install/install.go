package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/origin/pkg/quota/apiserver/admission/apis/runonceduration"
	"github.com/openshift/origin/pkg/quota/apiserver/admission/apis/runonceduration/v1"
)

func InstallLegacyInternal(scheme *runtime.Scheme) {
	utilruntime.Must(runonceduration.InstallLegacy(scheme))
	utilruntime.Must(v1.InstallLegacy(scheme))
}
