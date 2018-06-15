// +build acceptance compute availabilityzones

package v2

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/availabilityzones"
)

func TestAvailabilityZonesList(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	allPages, err := availabilityzones.List(client).AllPages()
	if err != nil {
		t.Fatalf("Unable to list availability zones info: %v", err)
	}

	availabilityZoneInfo, err := availabilityzones.ExtractAvailabilityZones(allPages)
	if err != nil {
		t.Fatalf("Unable to extract availability zones info: %v", err)
	}

	for _, zoneInfo := range availabilityZoneInfo {
		tools.PrintResource(t, zoneInfo)
	}
}

func TestAvailabilityZonesListDetail(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	allPages, err := availabilityzones.ListDetail(client).AllPages()
	if err != nil {
		t.Fatalf("Unable to list availability zones detailed info: %v", err)
	}

	availabilityZoneInfo, err := availabilityzones.ExtractAvailabilityZones(allPages)
	if err != nil {
		t.Fatalf("Unable to extract availability zones detailed info: %v", err)
	}

	for _, zoneInfo := range availabilityZoneInfo {
		tools.PrintResource(t, zoneInfo)
	}
}
