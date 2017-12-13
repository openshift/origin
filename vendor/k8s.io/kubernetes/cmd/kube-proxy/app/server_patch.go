package app

import (
	"github.com/spf13/pflag"

	"k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig"
)

func (o *Options) GetConfig() *kubeproxyconfig.KubeProxyConfiguration {
	return o.config
}

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	AddFlags(o, fs)
}
