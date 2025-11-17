package durations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/bigquery"
	"github.com/spf13/cobra"
	"google.golang.org/api/iterator"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"
)

func NewDurationsCommand(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDurationsOptions(streams)

	cmd := &cobra.Command{
		Use:   "durations",
		Short: "Generate test duration data from BigQuery",
		Long: templates.LongDesc(`
			Generate test duration data from BigQuery

			This command queries the BigQuery table containing junit test results and produces
			a JSON file with average test durations for tests in the openshift-tests suite
			from the last 7 days (configurable). Durations are calculated as the average
			and rounded to the nearest second.

			By default, the output is written to pkg/test/ginkgo/testDurations.json.

			The output format is:
			{
				"test.name": {
					"average_duration": 123
				}
			}
		`),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Run(context.Background()); err != nil {
				return err
			}
			return nil
		},
	}

	o.AddFlags(cmd.Flags())
	return cmd
}

// DurationResult represents a test duration result from BigQuery
type DurationResult struct {
	TestName        string  `bigquery:"test_name"`
	AverageDuration float64 `bigquery:"average_duration"`
}

// TestDurationData represents the output format for a single test
type TestDurationData struct {
	AverageDuration int `json:"average_duration"`
}

// Run executes the durations command
func (o *DurationsOptions) Run(ctx context.Context) error {
	// Create BigQuery client
	client, err := bigquery.NewClient(ctx, o.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to create BigQuery client: %w", err)
	}
	defer client.Close()

	// Build the SQL query
	query := o.buildQuery()

	// Execute the query
	q := client.Query(query)
	it, err := q.Read(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	// Collect results
	results := make(map[string]TestDurationData)
	for {
		var row DurationResult
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read query result: %w", err)
		}

		// Round to nearest second and store
		results[row.TestName] = TestDurationData{
			AverageDuration: int(row.AverageDuration),
		}
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Ensure the directory exists
	dir := fmt.Sprintf("%s", o.OutputFile[:strings.LastIndex(o.OutputFile, "/")])
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write output to file
	err = os.WriteFile(o.OutputFile, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}
	fmt.Fprintf(o.Out, "Test duration data written to %s\n", o.OutputFile)

	return nil
}

// buildQuery constructs the BigQuery SQL query
func (o *DurationsOptions) buildQuery() string {
	return fmt.Sprintf(`
SELECT 
  test_name,
  ROUND(AVG(duration_ms / 1000.0)) as average_duration
FROM `+"`%s.%s.%s`"+`
WHERE 
  modified_time >= DATETIME_SUB(CURRENT_DATETIME(), INTERVAL %d DAY)
  AND testsuite LIKE '%%openshift-tests%%'
GROUP BY test_name
ORDER BY test_name
	`, o.ProjectID, o.DatasetID, o.TableID, o.Days)
}
