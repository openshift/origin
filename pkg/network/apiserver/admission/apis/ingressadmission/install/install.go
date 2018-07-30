package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/origin/pkg/network/apiserver/admission/apis/ingressadmission"
	"github.com/openshift/origin/pkg/network/apiserver/admission/apis/ingressadmission/v1"
)

func InstallLegacyInternal(scheme *runtime.Scheme) {
	utilruntime.Must(ingressadmission.InstallLegacy(scheme))
	utilruntime.Must(v1.InstallLegacy(scheme))
}
