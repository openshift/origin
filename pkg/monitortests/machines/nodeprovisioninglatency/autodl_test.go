package nodeprovisioninglatency

import (
	"testing"
	"time"
)

func TestProvisioningRowMapColumnsMatchSchema(t *testing.T) {
	row := provisioningRowMap(provisioningRow{
		MachineName:      "worker-1",
		MachineRole:      roleWorker,
		CreatedDuringRun: true,
	})

	for column := range row {
		if _, ok := provisioningSchema()[column]; !ok {
			t.Errorf("autodl row column %q is missing from the schema", column)
		}
	}

	// Columns that are always present, even when the value is empty.
	for _, column := range []string{"MachineName", "MachineRole", "CreatedDuringRun", "InstanceExistsSource", "NodeReadySource"} {
		if _, ok := provisioningSchema()[column]; !ok {
			t.Errorf("schema is missing required column %q", column)
		}
		if _, ok := row[column]; !ok {
			t.Errorf("row is missing required column %q", column)
		}
	}
}

func TestProvisioningRowMapFormatting(t *testing.T) {
	timestamp := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	row := provisioningRowMap(provisioningRow{
		MachineName:              "worker-1",
		CreatedDuringRun:         true,
		MachineDeletedDuringRun:  false,
		MachineCreationTimestamp: timestamp,
		InstanceExistsSource:     sourceCondition,
		MachineToInstance:        durationPtr(90*time.Second + 500*time.Millisecond),
	})

	present := map[string]string{
		"MachineCreationTimestamp": timestamp.Format(time.RFC3339),
		"CreatedDuringRun":         "true",
		"MachineDeletedDuringRun":  "false",
		"MachineToInstanceSeconds": "90.500",
	}
	for column, want := range present {
		if got := row[column]; got != want {
			t.Errorf("%s = %q, want %q", column, got, want)
		}
	}

	absent := []string{
		"InstanceCreatedTimestamp",
		"NodeCreationTimestamp",
		"NodeReadyTimestamp",
		"InstanceToNodeSeconds",
		"NodeToReadySeconds",
		"MachineToNodeSeconds",
		"MachineToReadySeconds",
	}
	for _, column := range absent {
		if got, ok := row[column]; ok {
			t.Errorf("%s should be omitted, got %q", column, got)
		}
	}
}
