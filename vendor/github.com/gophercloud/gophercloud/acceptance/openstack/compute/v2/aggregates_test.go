// +build acceptance compute aggregates

package v2

import (
	"fmt"
	"testing"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/aggregates"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/hypervisors"
)

func TestAggregatesList(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	allPages, err := aggregates.List(client).AllPages()
	if err != nil {
		t.Fatalf("Unable to list aggregates: %v", err)
	}

	allAggregates, err := aggregates.ExtractAggregates(allPages)
	if err != nil {
		t.Fatalf("Unable to extract aggregates")
	}

	for _, h := range allAggregates {
		tools.PrintResource(t, h)
	}
}

func TestAggregatesCreateDelete(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	createdAggregate, err := CreateAggregate(t, client)
	if err != nil {
		t.Fatalf("Unable to create an aggregate: %v", err)
	}
	defer DeleteAggregate(t, client, createdAggregate)

	tools.PrintResource(t, createdAggregate)
}

func TestAggregatesGet(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	createdAggregate, err := CreateAggregate(t, client)
	if err != nil {
		t.Fatalf("Unable to create an aggregate: %v", err)
	}
	defer DeleteAggregate(t, client, createdAggregate)

	aggregate, err := aggregates.Get(client, createdAggregate.ID).Extract()
	if err != nil {
		t.Fatalf("Unable to get an aggregate: %v", err)
	}

	tools.PrintResource(t, aggregate)
}

func TestAggregatesUpdate(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	createdAggregate, err := CreateAggregate(t, client)
	if err != nil {
		t.Fatalf("Unable to create an aggregate: %v", err)
	}
	defer DeleteAggregate(t, client, createdAggregate)

	updateOpts := aggregates.UpdateOpts{
		Name:             "new_aggregate_name",
		AvailabilityZone: "new_azone",
	}

	updatedAggregate, err := aggregates.Update(client, createdAggregate.ID, updateOpts).Extract()
	if err != nil {
		t.Fatalf("Unable to update an aggregate: %v", err)
	}

	tools.PrintResource(t, updatedAggregate)
}

func TestAggregatesAddRemoveHost(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	hostToAdd, err := getHypervisor(t, client)
	if err != nil {
		t.Fatal(err)
	}

	createdAggregate, err := CreateAggregate(t, client)
	if err != nil {
		t.Fatalf("Unable to create an aggregate: %v", err)
	}
	defer DeleteAggregate(t, client, createdAggregate)

	addHostOpts := aggregates.AddHostOpts{
		Host: hostToAdd.HypervisorHostname,
	}

	aggregateWithNewHost, err := aggregates.AddHost(client, createdAggregate.ID, addHostOpts).Extract()
	if err != nil {
		t.Fatalf("Unable to add host to aggregate: %v", err)
	}

	tools.PrintResource(t, aggregateWithNewHost)

	removeHostOpts := aggregates.RemoveHostOpts{
		Host: hostToAdd.HypervisorHostname,
	}

	aggregateWithRemovedHost, err := aggregates.RemoveHost(client, createdAggregate.ID, removeHostOpts).Extract()
	if err != nil {
		t.Fatalf("Unable to remove host from aggregate: %v", err)
	}

	tools.PrintResource(t, aggregateWithRemovedHost)
}

func TestAggregatesSetRemoveMetadata(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	createdAggregate, err := CreateAggregate(t, client)
	if err != nil {
		t.Fatalf("Unable to create an aggregate: %v", err)
	}
	defer DeleteAggregate(t, client, createdAggregate)

	opts := aggregates.SetMetadataOpts{
		Metadata: map[string]interface{}{"key": "value"},
	}

	aggregateWithMetadata, err := aggregates.SetMetadata(client, createdAggregate.ID, opts).Extract()
	if err != nil {
		t.Fatalf("Unable to set metadata to aggregate: %v", err)
	}

	tools.PrintResource(t, aggregateWithMetadata)

	optsToRemove := aggregates.SetMetadataOpts{
		Metadata: map[string]interface{}{"key": nil},
	}

	aggregateWithRemovedKey, err := aggregates.SetMetadata(client, createdAggregate.ID, optsToRemove).Extract()
	if err != nil {
		t.Fatalf("Unable to set metadata to aggregate: %v", err)
	}

	tools.PrintResource(t, aggregateWithRemovedKey)
}

func getHypervisor(t *testing.T, client *gophercloud.ServiceClient) (*hypervisors.Hypervisor, error) {
	allPages, err := hypervisors.List(client).AllPages()
	if err != nil {
		t.Fatalf("Unable to list hypervisors: %v", err)
	}

	allHypervisors, err := hypervisors.ExtractHypervisors(allPages)
	if err != nil {
		t.Fatal("Unable to extract hypervisors")
	}

	for _, h := range allHypervisors {
		return &h, nil
	}

	return nil, fmt.Errorf("Unable to get hypervisor")
}
