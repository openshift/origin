package ibmcloud

import (
	"k8s.io/kubernetes/test/e2e/framework"
)

const ProviderName = "ibmcloud"

func init() {
	framework.RegisterProvider(ProviderName, newProvider)
}

func newProvider() (framework.ProviderInterface, error) {
	return &Provider{}, nil
}

// Provider is a structure to handle IBMCloud for e2e testing
type Provider struct {
	framework.NullProvider
}
