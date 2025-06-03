package list

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"

	origincmd "github.com/openshift/origin/pkg/cmd"
	"github.com/openshift/origin/pkg/test/extensions"
	"github.com/openshift/origin/pkg/test/ginkgo"
	"github.com/openshift/origin/pkg/testsuites"
)

func NewListSuitesCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suites",
		Short: "List available test suites",
		Long: templates.LongDesc(`
		List all available test suites that can be run with the 'run' command.

		This command displays the names and descriptions of all test suites available
		in openshift-tests. Use the suite names with the 'run' command to execute
		specific test suites.
		`),
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(c *cobra.Command, args []string) error {
			if len(os.Getenv("OPENSHIFT_SKIP_EXTERNAL_TESTS")) == 0 {
				return origincmd.RequireClusterAccess(c, args)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get output format flag
			const flag = "output"
			outputFormat, err := cmd.Flags().GetString(flag)
			if err != nil {
				return errors.Wrapf(err, "error accessing flag %s for command %s", flag, cmd.Name())
			}

			suites := testsuites.InternalTestSuites()

			ctx := context.TODO()

			// Extract all test binaries from the release payload
			cleanup, binaries, err := extensions.ExtractAllTestBinaries(ctx, 10)
			if err != nil {
				return fmt.Errorf("failed to extract test binaries: %w", err)
			}
			defer cleanup()

			// Get info from all binaries
			extensionInfos, err := binaries.Info(ctx, 4)
			if err != nil {
				return fmt.Errorf("failed to get extension info: %w", err)
			}

			for _, e := range extensionInfos {
				for _, s := range e.Suites {
					suites = append(suites, &ginkgo.TestSuite{
						Name:        s.Name,
						Description: s.Description,
						Extension:   e,
						Matches: func(name string) bool {
							return name == s.Name
						},
					})
				}
			}

			// Output in the requested format
			switch outputFormat {
			case "":
				// Default human-readable format
				output := testsuites.SuitesString(suites, "Available test suites:\n\n")
				fmt.Fprint(streams.Out, output)
			case "json":
				jsonData, err := json.MarshalIndent(suites, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal suites to JSON: %w", err)
				}
				fmt.Fprintln(streams.Out, string(jsonData))
			case "yaml":
				yamlData, err := yaml.Marshal(suites)
				if err != nil {
					return fmt.Errorf("failed to marshal suites to YAML: %w", err)
				}
				fmt.Fprintln(streams.Out, string(yamlData))
			default:
				return errors.Errorf("invalid output format: %s", outputFormat)
			}

			return nil
		},
	}

	cmd.Flags().StringP("output", "o", "", "Output format; available options are 'yaml' and 'json'")
	return cmd
}
