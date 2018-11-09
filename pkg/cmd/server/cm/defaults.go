package cm

import (
	apiserverflag "k8s.io/apiserver/pkg/util/flag"
	kcmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kcmoptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
)

func OriginControllerManagerAddFlags(cmserver *kcmoptions.KubeControllerManagerOptions) apiserverflag.NamedFlagSets {
	return cmserver.Flags(kcmapp.KnownControllers(), kcmapp.ControllersDisabledByDefault.List())
}
