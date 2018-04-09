package app

import (
	"k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig"
)

func (o *Options) GetConfig() *kubeproxyconfig.KubeProxyConfiguration {
	return o.config
}
