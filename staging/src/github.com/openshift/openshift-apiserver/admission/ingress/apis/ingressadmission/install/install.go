package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/openshift-apiserver/admission/ingress/apis/ingressadmission"
	"github.com/openshift/openshift-apiserver/admission/ingress/apis/ingressadmission/v1"
)

func InstallInternal(scheme *runtime.Scheme) {
	utilruntime.Must(ingressadmission.Install(scheme))
	utilruntime.Must(v1.Install(scheme))
}
