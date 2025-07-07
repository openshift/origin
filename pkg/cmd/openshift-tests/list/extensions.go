package list

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"

	origincmd "github.com/openshift/origin/pkg/cmd"
	"github.com/openshift/origin/pkg/test/extensions"
)

func NewListExtensionsCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extensions",
		Short: "List available test extensions",
		Long: templates.LongDesc(`
		List all available test extensions that provide additional tests.

		This command extracts and queries all external test binaries from the release
		payload to display information about available extensions, including their
		components, source information, and advertised test suites.
		`),
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE:       origincmd.RequireClusterAccess,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if len(os.Getenv("OPENSHIFT_SKIP_EXTERNAL_TESTS")) > 0 {
				return fmt.Errorf("OPENSHIFT_SKIP_EXTERNAL_TESTS is set, cannot list extensions")
			}

			// Get output format flag
			const flag = "output"
			outputFormat, err := cmd.Flags().GetString(flag)
			if err != nil {
				return errors.Wrapf(err, "error accessing flag %s for command %s", flag, cmd.Name())
			}

			// Extract all test binaries from the release payload
			cleanup, binaries, err := extensions.ExtractAllTestBinaries(ctx, 10)
			if err != nil {
				return fmt.Errorf("failed to extract test binaries: %w", err)
			}
			defer cleanup()

			if len(binaries) == 0 {
				switch outputFormat {
				case "json":
					fmt.Fprint(streams.Out, "[]\n")
				case "yaml":
					fmt.Fprint(streams.Out, "[]\n")
				default:
					fmt.Fprint(streams.Out, "No test extensions found.\n")
				}
				return nil
			}

			// Get info from all binaries
			logrus.Infof("Fetching info from %d extension binaries", len(binaries))
			infos, err := binaries.Info(ctx, 4)
			if err != nil {
				logrus.Errorf("Failed to get extension info: %v", err)
				return fmt.Errorf("failed to get extension info: %w", err)
			}

			// Output in the requested format
			switch outputFormat {
			case "":
				// Default human-readable format
				fmt.Fprintf(streams.Out, "Available test extensions:\n\n")
				for _, info := range infos {
					printExtensionInfo(streams.Out, info)
				}
			case "json":
				jsonData, err := json.MarshalIndent(infos, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal extensions to JSON: %w", err)
				}
				fmt.Fprintln(streams.Out, string(jsonData))
			case "yaml":
				yamlData, err := yaml.Marshal(infos)
				if err != nil {
					return fmt.Errorf("failed to marshal extensions to YAML: %w", err)
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

func printExtensionInfo(out io.Writer, info *extensions.Extension) {
	fmt.Fprintf(out, "%s:%s:%s\n", info.Component.Product, info.Component.Kind, info.Component.Name)
	fmt.Fprintf(out, "  API Version: %s\n", info.APIVersion)

	if info.Source.SourceBinary != "" {
		fmt.Fprintf(out, "  Binary: %s\n", info.Source.SourceBinary)
	}

	if info.Source.SourceImage != "" {
		fmt.Fprintf(out, "  Source Image: %s\n", info.Source.SourceImage)
	}

	if info.Source.SourceURL != "" {
		fmt.Fprintf(out, "  Source URL: %s\n", info.Source.SourceURL)
	}

	if info.Source.Commit != "" {
		fmt.Fprintf(out, "  Commit: %s", info.Source.Commit)
		if info.Source.GitTreeState != "" && info.Source.GitTreeState != "clean" {
			fmt.Fprintf(out, " (%s)", info.Source.GitTreeState)
		}
		fmt.Fprintf(out, "\n")
	}

	if info.Source.BuildDate != "" {
		fmt.Fprintf(out, "  Build Date: %s\n", info.Source.BuildDate)
	}

	if len(info.Suites) > 0 {
		fmt.Fprintf(out, "  Advertised Suites:\n")
		for _, suite := range info.Suites {
			fmt.Fprintf(out, "    - %s", suite.Name)
			if len(suite.Parents) > 0 {
				fmt.Fprintf(out, " (parents: %s)", strings.Join(suite.Parents, ", "))
			}
			fmt.Fprintf(out, "\n")
		}
	}

	fmt.Fprintf(out, "\n")
}
