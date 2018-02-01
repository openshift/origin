package validate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation"
)

const (
	ValidateNodeConfigRecommendedName    = "node-config"
	validateNodeConfigDeprecationMessage = `This command is deprecated and will be removed. Use 'oc adm diagnostics NodeConfigCheck --node-config=path/to/config.yaml' instead.`
)

var (
	validateNodeConfigLong = templates.LongDesc(`
		Validate the configuration file for a node.

		This command validates that a configuration file intended to be used for a node is valid.`)

	validateNodeConfigExample = templates.Examples(`
		# Validate node configuration file
  	%s openshift.local.config/master/node-config.yaml`)
)

type ValidateNodeConfigOptions struct {
	// NodeConfigFile is the location of the config file to be validated
	NodeConfigFile string

	// Out is the writer to write output to
	Out io.Writer
}

// NewCommandValidateMasterConfig provides a CLI handler for the `validate all-in-one` command
func NewCommandValidateNodeConfig(name, fullName string, out io.Writer) *cobra.Command {
	options := &ValidateNodeConfigOptions{
		Out: out,
	}

	cmd := &cobra.Command{
		Use:        fmt.Sprintf("%s SOURCE", name),
		Short:      "Validate the configuration file for a node",
		Long:       validateNodeConfigLong,
		Example:    fmt.Sprintf(validateNodeConfigExample, fullName),
		Deprecated: validateNodeConfigDeprecationMessage,
		Run: func(c *cobra.Command, args []string) {
			if err := options.Complete(args); err != nil {
				cmdutil.CheckErr(cmdutil.UsageErrorf(c, err.Error()))
			}

			ok, err := options.Run()
			cmdutil.CheckErr(err)
			if !ok {
				fmt.Fprintf(options.Out, "FAILURE: Validation failed for file: %s\n", options.NodeConfigFile)
				os.Exit(1)
			}

			fmt.Fprintf(options.Out, "SUCCESS: Validation succeeded for file: %s\n", options.NodeConfigFile)
		},
	}

	return cmd
}

func (o *ValidateNodeConfigOptions) Complete(args []string) error {
	if len(args) != 1 {
		return errors.New("exactly one source file is required")
	}
	o.NodeConfigFile = args[0]
	return nil
}

// Run runs the node config validation and returns the result of the validation as a boolean as well as any errors
// that occurred trying to validate the file
func (o *ValidateNodeConfigOptions) Run() (ok bool, err error) {
	nodeConfig, err := configapilatest.ReadAndResolveNodeConfig(o.NodeConfigFile)
	if err != nil {
		return true, err
	}

	results := validation.ValidateNodeConfig(nodeConfig, nil)
	writer := tabwriter.NewWriter(o.Out, minColumnWidth, tabWidth, padding, padchar, flags)
	err = prettyPrintValidationResults(results, writer)
	if err != nil {
		return len(results.Errors) == 0, fmt.Errorf("could not print results: %v", err)
	}
	writer.Flush()
	return len(results.Errors) == 0, nil
}
