package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/origin/pkg/project/apiserver/admission/apis/requestlimit"
	"github.com/openshift/origin/pkg/project/apiserver/admission/apis/requestlimit/v1"
)

func InstallInternal(scheme *runtime.Scheme) {
	utilruntime.Must(requestlimit.Install(scheme))
	utilruntime.Must(v1.Install(scheme))
}
