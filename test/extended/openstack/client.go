package openstack

import (
	"os"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

const (
	serviceCompute = "compute"
)

// client generates a Gophercloud client for the given service. Available
// services are exposed from this file as constants.
func client(service string) (*gophercloud.ServiceClient, error) {
	opts := &clientconfig.ClientOpts{
		Cloud: os.Getenv("OS_CLOUD"),
	}
	return clientconfig.NewServiceClient(service, opts)
}
