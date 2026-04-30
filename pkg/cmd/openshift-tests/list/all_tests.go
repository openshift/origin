package list

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift/origin/pkg/test/extensions"
	"github.com/openshift/origin/pkg/testsuites"
)

// resolveSuiteQualifiers finds a suite by name, first checking origin's internal
// suites, then checking suites advertised by the already-extracted extension binaries.
func resolveSuiteQualifiers(ctx context.Context, suiteName string, binaries extensions.TestBinaries) ([]string, error) {
	for _, s := range testsuites.InternalTestSuites() {
		if s.Name == suiteName {
			return s.Qualifiers, nil
		}
	}

	extensionInfos, err := binaries.Info(ctx, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to get extension info: %w", err)
	}
	for _, e := range extensionInfos {
		for _, s := range e.Suites {
			if s.Name == suiteName {
				return s.Qualifiers, nil
			}
		}
	}

	return nil, fmt.Errorf("suite %q not found", suiteName)
}

func NewListAllTestsCommand(streams genericclioptions.IOStreams) *cobra.Command {
	var suiteName string

	cmd := &cobra.Command{
		Use:   "all-tests",
		Short: "List tests from all extension binaries",
		Long: templates.LongDesc(`
		List all tests discovered from all extension binaries in the release payload.

		Unlike 'list tests', which lists tests from a single extension component,
		this command aggregates tests from all extension binaries — the same set
		of tests that 'run' operates on.

		Use --suite to filter tests by a suite's qualifiers. This works with both
		origin-defined suites (like openshift/network/third-party) and suites
		advertised by extension binaries.

		This command does not require cluster access.
		`),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			const defaultBinaryParallelism = 10

			extractionContext, extractionContextCancel := context.WithTimeout(ctx, 30*60*1e9)
			defer extractionContextCancel()
			cleanUpFn, allBinaries, _, err := extensions.ExtractAllTestBinaries(extractionContext, defaultBinaryParallelism, extensions.WithPayloadOnly())
			if err != nil {
				return fmt.Errorf("failed to extract test binaries: %w", err)
			}
			defer cleanUpFn()

			infoContext, infoContextCancel := context.WithTimeout(ctx, 30*60*1e9)
			defer infoContextCancel()
			logrus.Infof("Fetching info from %d extension binaries", len(allBinaries))
			if _, err := allBinaries.Info(infoContext, defaultBinaryParallelism); err != nil {
				logrus.Warnf("Some extension binaries failed info fetch (they may require cluster access): %v", err)
			}

			// Filter to binaries that successfully returned info, since ListTests
			// requires info to be populated. Binaries that failed info (e.g. due to
			// missing cluster access) are skipped.
			var availableBinaries extensions.TestBinaries
			for _, b := range allBinaries {
				if b.HasInfo() {
					availableBinaries = append(availableBinaries, b)
				}
			}
			logrus.Infof("%d of %d binaries available for listing", len(availableBinaries), len(allBinaries))

			listContext, listContextCancel := context.WithTimeout(ctx, 10*60*1e9)
			defer listContextCancel()

			specs, err := availableBinaries.ListTests(listContext, defaultBinaryParallelism, nil)
			if err != nil {
				return fmt.Errorf("failed to list tests: %w", err)
			}

			logrus.Infof("Discovered %d total tests", len(specs))

			if suiteName != "" {
				qualifiers, err := resolveSuiteQualifiers(ctx, suiteName, availableBinaries)
				if err != nil {
					return err
				}

				specs, err = extensions.FilterWrappedSpecs(specs, qualifiers)
				if err != nil {
					return fmt.Errorf("failed to filter tests by suite qualifiers: %w", err)
				}

				logrus.Infof("Suite %q selected %d tests", suiteName, len(specs))
			}

			outputFormat, err := cmd.Flags().GetString("output")
			if err != nil {
				return errors.Wrapf(err, "error accessing flag output for command %s", cmd.Name())
			}

			sort.Slice(specs, func(i, j int) bool {
				return specs[i].Name < specs[j].Name
			})

			switch outputFormat {
			case "":
				for _, spec := range specs {
					fmt.Fprintln(streams.Out, spec.Name)
				}
			case "json":
				data, err := json.MarshalIndent(specs, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal tests to JSON: %w", err)
				}
				fmt.Fprintln(streams.Out, string(data))
			case "yaml":
				data, err := yaml.Marshal(specs)
				if err != nil {
					return fmt.Errorf("failed to marshal tests to YAML: %w", err)
				}
				fmt.Fprintln(streams.Out, string(data))
			default:
				return fmt.Errorf("invalid output format: %s", outputFormat)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&suiteName, "suite", "", "Filter tests by the qualifiers of the specified suite")
	cmd.Flags().StringP("output", "o", "", "Output format; available options are 'yaml' and 'json'")
	return cmd
}
