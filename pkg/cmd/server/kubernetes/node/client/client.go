package client

import (
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
)

func GetKubeletClientConfig(options configapi.MasterConfig) *kubeletclient.KubeletClientConfig {
	config := &kubeletclient.KubeletClientConfig{
		Port: options.KubeletClientInfo.Port,
		PreferredAddressTypes: []string{
			string("Hostname"),
			string("InternalIP"),
			string("ExternalIP"),
		},
	}

	if len(options.KubeletClientInfo.CA) > 0 {
		config.EnableHttps = true
		config.CAFile = options.KubeletClientInfo.CA
	}

	if len(options.KubeletClientInfo.ClientCert.CertFile) > 0 {
		config.EnableHttps = true
		config.CertFile = options.KubeletClientInfo.ClientCert.CertFile
		config.KeyFile = options.KubeletClientInfo.ClientCert.KeyFile
	}

	return config
}
