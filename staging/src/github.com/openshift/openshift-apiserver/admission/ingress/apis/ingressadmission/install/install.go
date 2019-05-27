package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/origin/pkg/route/apiserver/admission/apis/ingressadmission"
	"github.com/openshift/origin/pkg/route/apiserver/admission/apis/ingressadmission/v1"
)

func InstallInternal(scheme *runtime.Scheme) {
	utilruntime.Must(ingressadmission.Install(scheme))
	utilruntime.Must(v1.Install(scheme))
}
