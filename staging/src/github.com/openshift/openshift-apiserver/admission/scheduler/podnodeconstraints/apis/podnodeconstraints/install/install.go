package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/openshift-apiserver/admission/scheduler/podnodeconstraints/apis/podnodeconstraints"
	"github.com/openshift/openshift-apiserver/admission/scheduler/podnodeconstraints/apis/podnodeconstraints/v1"
)

func InstallInternal(scheme *runtime.Scheme) {
	utilruntime.Must(podnodeconstraints.Install(scheme))
	utilruntime.Must(v1.Install(scheme))
}
