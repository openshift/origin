package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/openshift-apiserver/admission/requestlimit/apis/requestlimit"
	"github.com/openshift/openshift-apiserver/admission/requestlimit/apis/requestlimit/v1"
)

func InstallInternal(scheme *runtime.Scheme) {
	utilruntime.Must(requestlimit.Install(scheme))
	utilruntime.Must(v1.Install(scheme))
}
