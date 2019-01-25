package app

import (
	kubeproxyconfig "k8s.io/kubernetes/pkg/proxy/apis/config"
)

func (o *Options) GetConfig() *kubeproxyconfig.KubeProxyConfiguration {
	return o.config
}
