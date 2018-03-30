// +build acceptance compute servers

package v2

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/migrate"
)

func TestMigrate(t *testing.T) {
	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	server, err := CreateServer(t, client)
	if err != nil {
		t.Fatalf("Unable to create server: %v", err)
	}
	defer DeleteServer(t, client, server)

	t.Logf("Attempting to migrate server %s", server.ID)

	err = migrate.Migrate(client, server.ID).ExtractErr()
	if err != nil {
		t.Fatalf("Error during migration: %v", err)
	}
}

func TestLiveMigrate(t *testing.T) {
	choices, err := clients.AcceptanceTestChoicesFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	if !choices.LiveMigrate {
		t.Skip("Testing of live migration is disabled")
	}

	client, err := clients.NewComputeV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	server, err := CreateServer(t, client)
	if err != nil {
		t.Fatalf("Unable to create server: %v", err)
	}
	defer DeleteServer(t, client, server)

	t.Logf("Attempting to migrate server %s", server.ID)

	blockMigration := false
	diskOverCommit := false

	liveMigrateOpts := migrate.LiveMigrateOpts{
		BlockMigration: &blockMigration,
		DiskOverCommit: &diskOverCommit,
	}

	err = migrate.LiveMigrate(client, server.ID, liveMigrateOpts).ExtractErr()
	if err != nil {
		t.Fatalf("Error during live migration: %v", err)
	}
}
