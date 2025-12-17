package durations

import (
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// DurationsOptions contains the options for the durations command
type DurationsOptions struct {
	// BigQuery configuration
	ProjectID string
	DatasetID string
	TableID   string

	// Query configuration
	Days int

	// Output configuration
	OutputFile string

	// IO streams
	genericclioptions.IOStreams
}

// NewDurationsOptions creates a new DurationsOptions with default values
func NewDurationsOptions(streams genericclioptions.IOStreams) *DurationsOptions {
	return &DurationsOptions{
		ProjectID:  "openshift-gce-devel",
		DatasetID:  "ci_analysis_us",
		TableID:    "junit",
		Days:       7,
		OutputFile: "pkg/test/ginkgo/testDurations.json",
		IOStreams:  streams,
	}
}

// Complete completes the options based on command arguments
func (o *DurationsOptions) Complete(args []string) error {
	// No arguments expected for this command
	if len(args) > 0 {
		return fmt.Errorf("no arguments are expected")
	}
	return nil
}

// Validate validates the options
func (o *DurationsOptions) Validate() error {
	if o.ProjectID == "" {
		return fmt.Errorf("project ID cannot be empty")
	}
	if o.DatasetID == "" {
		return fmt.Errorf("dataset ID cannot be empty")
	}
	if o.TableID == "" {
		return fmt.Errorf("table ID cannot be empty")
	}
	if o.Days <= 0 {
		return fmt.Errorf("days must be a positive integer")
	}
	if o.Days > 365 {
		return fmt.Errorf("days cannot exceed 365")
	}
	return nil
}
