// +build acceptance compute services

package v2

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/services"
)

func TestServicesList(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	allPages, err := services.List(client).AllPages()
	if err != nil {
		t.Fatalf("Unable to list services: %v", err)
	}

	allServices, err := services.ExtractServices(allPages)
	if err != nil {
		t.Fatalf("Unable to extract services")
	}

	for _, service := range allServices {
		tools.PrintResource(t, service)
	}
}
