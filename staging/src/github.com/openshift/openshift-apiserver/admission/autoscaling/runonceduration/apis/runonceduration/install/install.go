package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/openshift-apiserver/admission/autoscaling/runonceduration/apis/runonceduration"
	"github.com/openshift/openshift-apiserver/admission/autoscaling/runonceduration/apis/runonceduration/v1"
)

func InstallInternal(scheme *runtime.Scheme) {
	utilruntime.Must(runonceduration.Install(scheme))
	utilruntime.Must(v1.Install(scheme))
}
