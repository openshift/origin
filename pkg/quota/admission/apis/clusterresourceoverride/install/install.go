package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/origin/pkg/quota/admission/apis/clusterresourceoverride"
	"github.com/openshift/origin/pkg/quota/admission/apis/clusterresourceoverride/v1"
)

func InstallLegacyInternal(scheme *runtime.Scheme) {
	utilruntime.Must(clusterresourceoverride.InstallLegacy(scheme))
	utilruntime.Must(v1.InstallLegacy(scheme))
}
