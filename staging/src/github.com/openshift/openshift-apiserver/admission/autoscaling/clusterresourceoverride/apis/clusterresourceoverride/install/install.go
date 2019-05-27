package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/openshift-apiserver/admission/autoscaling/clusterresourceoverride/apis/clusterresourceoverride"
	"github.com/openshift/openshift-apiserver/admission/autoscaling/clusterresourceoverride/apis/clusterresourceoverride/v1"
)

func InstallInternal(scheme *runtime.Scheme) {
	utilruntime.Must(clusterresourceoverride.Install(scheme))
	utilruntime.Must(v1.Install(scheme))
}
