package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/origin/pkg/image/apiserver/admission/apis/imagepolicy"
	"github.com/openshift/origin/pkg/image/apiserver/admission/apis/imagepolicy/v1"
)

func InstallLegacyInternal(scheme *runtime.Scheme) {
	utilruntime.Must(imagepolicy.InstallLegacy(scheme))
	utilruntime.Must(v1.InstallLegacy(scheme))
}
