package nutanix

import (
	"k8s.io/kubernetes/test/e2e/framework"
)

func init() {
	framework.RegisterProvider("nutanix", newProvider)
}

func newProvider() (framework.ProviderInterface, error) {
	return &Provider{}, nil
}

// Structure to handle nutanix for e2e testing
type Provider struct {
	framework.NullProvider
}
