package nodeprovisioninglatency

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/openshift/origin/pkg/dataloader"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
)

const autodlTableName = "node_provisioning_latency"

// provisioningSchema lists every column in a stable order. The autodl loader ignores schema
// changes after the BigQuery table is created, so columns must not be renamed or reordered.
func provisioningSchema() map[string]dataloader.DataType {
	return map[string]dataloader.DataType{
		"MachineName":              dataloader.DataTypeString,
		"MachineSet":               dataloader.DataTypeString,
		"MachineRole":              dataloader.DataTypeString,
		"InstanceType":             dataloader.DataTypeString,
		"MachinePhase":             dataloader.DataTypeString,
		"CreatedDuringRun":         dataloader.DataTypeString,
		"MachineDeletedDuringRun":  dataloader.DataTypeString,
		"MachineCreationTimestamp": dataloader.DataTypeTimestamp,
		"InstanceCreatedTimestamp": dataloader.DataTypeTimestamp,
		"InstanceExistsSource":     dataloader.DataTypeString,
		"NodeCreationTimestamp":    dataloader.DataTypeTimestamp,
		"NodeReadyTimestamp":       dataloader.DataTypeTimestamp,
		"NodeReadySource":          dataloader.DataTypeString,
		"NodeName":                 dataloader.DataTypeString,
		"MachineToInstanceSeconds": dataloader.DataTypeFloat64,
		"InstanceToNodeSeconds":    dataloader.DataTypeFloat64,
		"NodeToReadySeconds":       dataloader.DataTypeFloat64,
		"MachineToNodeSeconds":     dataloader.DataTypeFloat64,
		"MachineToReadySeconds":    dataloader.DataTypeFloat64,
	}
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func formatDuration(d *time.Duration) string {
	if d == nil {
		return ""
	}
	return strconv.FormatFloat(d.Seconds(), 'f', 3, 64)
}

func formatBool(value bool) string {
	return strconv.FormatBool(value)
}

// provisioningRowMap omits timestamps and durations that are not available. autodl has no null
// type, and the loader's handling of "" for FLOAT64/TIMESTAMP columns is not guaranteed; the
// schema documents columns as optional, so absence is the safe encoding.
func provisioningRowMap(row provisioningRow) map[string]string {
	values := map[string]string{
		"MachineName":             row.MachineName,
		"MachineSet":              row.MachineSet,
		"MachineRole":             row.MachineRole,
		"InstanceType":            row.InstanceType,
		"MachinePhase":            row.MachinePhase,
		"CreatedDuringRun":        formatBool(row.CreatedDuringRun),
		"MachineDeletedDuringRun": formatBool(row.MachineDeletedDuringRun),
		"InstanceExistsSource":    row.InstanceExistsSource,
		"NodeReadySource":         row.NodeReadySource,
		"NodeName":                row.NodeName,
	}

	optional := map[string]string{
		"MachineCreationTimestamp": formatTimestamp(row.MachineCreationTimestamp),
		"InstanceCreatedTimestamp": formatTimestamp(row.InstanceCreatedTimestamp),
		"NodeCreationTimestamp":    formatTimestamp(row.NodeCreationTimestamp),
		"NodeReadyTimestamp":       formatTimestamp(row.NodeReadyTimestamp),
		"MachineToInstanceSeconds": formatDuration(row.MachineToInstance),
		"InstanceToNodeSeconds":    formatDuration(row.InstanceToNode),
		"NodeToReadySeconds":       formatDuration(row.NodeToReady),
		"MachineToNodeSeconds":     formatDuration(row.MachineToNode),
		"MachineToReadySeconds":    formatDuration(row.MachineToReady),
	}
	for column, value := range optional {
		if value != "" {
			values[column] = value
		}
	}

	return values
}

func provisioningDataFile(rows []provisioningRow) dataloader.DataFile {
	converted := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		converted = append(converted, provisioningRowMap(row))
	}

	return dataloader.DataFile{
		TableName: autodlTableName,
		Schema:    provisioningSchema(),
		Rows:      converted,
	}
}

func (m *nodeProvisioningLatency) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	rows := m.rows
	if len(rows) == 0 {
		m.logger.Printf("no node provisioning rows to export")
		return nil
	}

	fileName := filepath.Join(storageDir, fmt.Sprintf("node-provisioning-latency%s-%s", timeSuffix, dataloader.AutoDataLoaderSuffix))
	if err := dataloader.WriteDataFile(fileName, provisioningDataFile(rows)); err != nil {
		return fmt.Errorf("failed to write node provisioning autodl file %s: %w", fileName, err)
	}

	m.logger.Printf("wrote %d node provisioning rows to %s", len(rows), fileName)
	return nil
}

var _ monitortestframework.MonitorTest = &nodeProvisioningLatency{}
