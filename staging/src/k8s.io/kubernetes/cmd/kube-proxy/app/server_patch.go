package app

import (
	"github.com/spf13/pflag"

	"k8s.io/kubernetes/pkg/apis/componentconfig"
)

func (o *Options) GetConfig() *componentconfig.KubeProxyConfiguration {
	return o.config
}

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	AddFlags(o, fs)
}
