// +build acceptance compute flavors

package v2

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"

	identity "github.com/gophercloud/gophercloud/acceptance/openstack/identity/v3"
)

func TestFlavorsList(t *testing.T) {
	t.Logf("** Default flavors (same as Project flavors): **")
	t.Logf("")
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	allPages, err := flavors.ListDetail(client, nil).AllPages()
	if err != nil {
		t.Fatalf("Unable to retrieve flavors: %v", err)
	}

	allFlavors, err := flavors.ExtractFlavors(allPages)
	if err != nil {
		t.Fatalf("Unable to extract flavor results: %v", err)
	}

	for _, flavor := range allFlavors {
		tools.PrintResource(t, flavor)
	}

	flavorAccessTypes := [3]flavors.AccessType{flavors.PublicAccess, flavors.PrivateAccess, flavors.AllAccess}
	for _, flavorAccessType := range flavorAccessTypes {
		t.Logf("** %s flavors: **", flavorAccessType)
		t.Logf("")
		allPages, err := flavors.ListDetail(client, flavors.ListOpts{AccessType: flavorAccessType}).AllPages()
		if err != nil {
			t.Fatalf("Unable to retrieve flavors: %v", err)
		}

		allFlavors, err := flavors.ExtractFlavors(allPages)
		if err != nil {
			t.Fatalf("Unable to extract flavor results: %v", err)
		}

		for _, flavor := range allFlavors {
			tools.PrintResource(t, flavor)
			t.Logf("")
		}
	}

}

func TestFlavorsGet(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	choices, err := clients.AcceptanceTestChoicesFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	flavor, err := flavors.Get(client, choices.FlavorID).Extract()
	if err != nil {
		t.Fatalf("Unable to get flavor information: %v", err)
	}

	tools.PrintResource(t, flavor)
}

func TestFlavorCreateDelete(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	flavor, err := CreateFlavor(t, client)
	if err != nil {
		t.Fatalf("Unable to create flavor: %v", err)
	}
	defer DeleteFlavor(t, client, flavor)

	tools.PrintResource(t, flavor)
}

func TestFlavorAccessesList(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	flavor, err := CreatePrivateFlavor(t, client)
	if err != nil {
		t.Fatalf("Unable to create flavor: %v", err)
	}
	defer DeleteFlavor(t, client, flavor)

	allPages, err := flavors.ListAccesses(client, flavor.ID).AllPages()
	if err != nil {
		t.Fatalf("Unable to list flavor accesses: %v", err)
	}

	allAccesses, err := flavors.ExtractAccesses(allPages)
	if err != nil {
		t.Fatalf("Unable to extract accesses: %v", err)
	}

	for _, access := range allAccesses {
		tools.PrintResource(t, access)
	}
}

func TestFlavorAccessCRUD(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	identityClient, err := clients.NewIdentityV3Client()
	if err != nil {
		t.Fatal("Unable to create identity client: %v", err)
	}

	project, err := identity.CreateProject(t, identityClient, nil)
	if err != nil {
		t.Fatal("Unable to create project: %v", err)
	}
	defer identity.DeleteProject(t, identityClient, project.ID)

	flavor, err := CreatePrivateFlavor(t, client)
	if err != nil {
		t.Fatalf("Unable to create flavor: %v", err)
	}
	defer DeleteFlavor(t, client, flavor)

	addAccessOpts := flavors.AddAccessOpts{
		Tenant: project.ID,
	}

	accessList, err := flavors.AddAccess(client, flavor.ID, addAccessOpts).Extract()
	if err != nil {
		t.Fatalf("Unable to add access to flavor: %v", err)
	}

	for _, access := range accessList {
		tools.PrintResource(t, access)
	}

	removeAccessOpts := flavors.RemoveAccessOpts{
		Tenant: project.ID,
	}

	accessList, err = flavors.RemoveAccess(client, flavor.ID, removeAccessOpts).Extract()
	if err != nil {
		t.Fatalf("Unable to remove access to flavor: %v", err)
	}

	for _, access := range accessList {
		tools.PrintResource(t, access)
	}
}

func TestFlavorExtraSpecsCRUD(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	flavor, err := CreatePrivateFlavor(t, client)
	if err != nil {
		t.Fatalf("Unable to create flavor: %v", err)
	}
	defer DeleteFlavor(t, client, flavor)

	createOpts := flavors.ExtraSpecsOpts{
		"hw:cpu_policy":        "CPU-POLICY",
		"hw:cpu_thread_policy": "CPU-THREAD-POLICY",
	}
	createdExtraSpecs, err := flavors.CreateExtraSpecs(client, flavor.ID, createOpts).Extract()
	if err != nil {
		t.Fatalf("Unable to create flavor extra_specs: %v", err)
	}
	tools.PrintResource(t, createdExtraSpecs)

	err = flavors.DeleteExtraSpec(client, flavor.ID, "hw:cpu_policy").ExtractErr()
	if err != nil {
		t.Fatalf("Unable to delete ExtraSpec: %v\n", err)
	}

	updateOpts := flavors.ExtraSpecsOpts{
		"hw:cpu_thread_policy": "CPU-THREAD-POLICY-BETTER",
	}
	updatedExtraSpec, err := flavors.UpdateExtraSpec(client, flavor.ID, updateOpts).Extract()
	if err != nil {
		t.Fatalf("Unable to update flavor extra_specs: %v", err)
	}
	tools.PrintResource(t, updatedExtraSpec)

	allExtraSpecs, err := flavors.ListExtraSpecs(client, flavor.ID).Extract()
	if err != nil {
		t.Fatalf("Unable to get flavor extra_specs: %v", err)
	}
	tools.PrintResource(t, allExtraSpecs)

	for key, _ := range allExtraSpecs {
		spec, err := flavors.GetExtraSpec(client, flavor.ID, key).Extract()
		if err != nil {
			t.Fatalf("Unable to get flavor extra spec: %v", err)
		}
		tools.PrintResource(t, spec)
	}

}
