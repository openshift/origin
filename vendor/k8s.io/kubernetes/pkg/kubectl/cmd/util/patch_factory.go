package util

import (
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// OpenShiftClientAccessFactory is a delta interface that we require for openshift on the factory.
type OpenShiftClientAccessFactory interface {
	RawConfig() (clientcmdapi.Config, error)
}

func (f *ring0Factory) RawConfig() (clientcmdapi.Config, error) {
	return f.clientConfig.RawConfig()
}
