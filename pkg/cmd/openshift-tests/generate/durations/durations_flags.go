package durations

import (
	"github.com/spf13/pflag"
)

// AddFlags adds command-line flags to the provided flagset
func (o *DurationsOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.ProjectID, "project", o.ProjectID, "BigQuery project ID")
	flags.StringVar(&o.DatasetID, "dataset", o.DatasetID, "BigQuery dataset ID")
	flags.StringVar(&o.TableID, "table", o.TableID, "BigQuery table ID")
	flags.IntVar(&o.Days, "days", o.Days, "Number of days to look back for test data")
	flags.StringVar(&o.OutputFile, "output", o.OutputFile, "Output file path")
}
