package cm

import (
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/util/sets"
	kcmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kcmoptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"

	origincontrollers "github.com/openshift/origin/pkg/cmd/server/origin/controller"
)

var (
	// default to the same controllers as upstream
	ControllersDisabledByDefault = kcmapp.ControllersDisabledByDefault

	// OriginalKubeControllers is a list of known controllers
	OriginalKubeControllers []string
)

// Register OpenShift controllers as known kubernetes controllers.
func init() {
	if len(OriginalKubeControllers) == 0 {
		OriginalKubeControllers = kcmapp.KnownControllers()
	}
	kcmapp.KnownControllersFn = KnownControllers
}

// KnownControllers returns controllers that are known to Openshift AND
// Kubernetes.
func KnownControllers() []string {
	ret := sets.StringKeySet(origincontrollers.NewOpenShiftControllerInitializers(nil))
	ret.Insert(OriginalKubeControllers...)
	return ret.List()
}

func OriginControllerManagerAddFlags(cmserver *kcmoptions.CMServer) func(flags *pflag.FlagSet) {
	return func(flags *pflag.FlagSet) {
		cmserver.AddFlags(flags, kcmapp.KnownControllers(), ControllersDisabledByDefault.List())
	}
}
