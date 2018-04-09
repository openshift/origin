// +build acceptance compute servers

package v2

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/attachinterfaces"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
)

func TestAttachDetachInterface(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	server, err := CreateServer(t, client)
	if err != nil {
		t.Fatalf("Unable to create server: %v", err)
	}

	defer DeleteServer(t, client, server)

	newServer, err := servers.Get(client, server.ID).Extract()
	if err != nil {
		t.Errorf("Unable to retrieve server: %v", err)
	}
	tools.PrintResource(t, newServer)

	intOpts := attachinterfaces.CreateOpts{}

	iface, err := attachinterfaces.Create(client, server.ID, intOpts).Extract()
	if err != nil {
		t.Fatal(err)
	}

	tools.PrintResource(t, iface)

	allPages, err := attachinterfaces.List(client, server.ID).AllPages()
	if err != nil {
		t.Fatal(err)
	}

	allIfaces, err := attachinterfaces.ExtractInterfaces(allPages)
	if err != nil {
		t.Fatal(err)
	}

	for _, i := range allIfaces {
		tools.PrintResource(t, i)
	}

	err = attachinterfaces.Delete(client, server.ID, iface.PortID).ExtractErr()
	if err != nil {
		t.Fatal(err)
	}
}
