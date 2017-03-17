package cm

import (
	"github.com/spf13/pflag"

	kcmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kcmoptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
)

var (
	// default to the same controllers as upstream
	ControllersDisabledByDefault = kcmapp.ControllersDisabledByDefault
)

func OriginControllerManagerAddFlags(flags *pflag.FlagSet) {
	kcmoptions.NewCMServer().AddFlags(flags, kcmapp.KnownControllers(), ControllersDisabledByDefault.List())
}
