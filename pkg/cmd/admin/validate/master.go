package validate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util/validation/field"

	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/templates"

	"github.com/openshift/origin/pkg/cmd/server/api/validation"
)

const (
	ValidateMasterConfigRecommendedName    = "master-config"
	validateMasterConfigDeprecationMessage = `This command is deprecated and will be removed. Use 'oadm diagnostics MasterConfigCheck --master-config=path/to/config.yaml' instead.`
)

var (
	validateMasterConfigLong = templates.LongDesc(`
		Validate the configuration file for a master server.

		This command validates that a configuration file intended to be used for a master server is valid.`)

	validateMasterConfigExample = templates.Examples(`
		# Validate master server configuration file
		%s openshift.local.config/master/master-config.yaml`)
)

type ValidateMasterConfigOptions struct {
	// MasterConfigFile is the location of the config file to be validated
	MasterConfigFile string

	// Out is the writer to write output to
	Out io.Writer
}

// NewCommandValidateMasterConfig provides a CLI handler for the `validate all-in-one` command
func NewCommandValidateMasterConfig(name, fullName string, out io.Writer) *cobra.Command {
	options := &ValidateMasterConfigOptions{
		Out: out,
	}

	cmd := &cobra.Command{
		Use:        fmt.Sprintf("%s SOURCE", name),
		Short:      "Validate the configuration file for a master server",
		Long:       validateMasterConfigLong,
		Example:    fmt.Sprintf(validateMasterConfigExample, fullName),
		Deprecated: validateMasterConfigDeprecationMessage,
		Run: func(c *cobra.Command, args []string) {
			if err := options.Complete(args); err != nil {
				cmdutil.CheckErr(cmdutil.UsageError(c, err.Error()))
			}

			ok, err := options.Run()
			cmdutil.CheckErr(err)
			if !ok {
				fmt.Fprintf(options.Out, "FAILURE: Validation failed for file: %s\n", options.MasterConfigFile)
				os.Exit(1)
			}

			fmt.Fprintf(options.Out, "SUCCESS: Validation succeeded for file: %s\n", options.MasterConfigFile)
		},
	}

	return cmd
}

func (o *ValidateMasterConfigOptions) Complete(args []string) error {
	if len(args) != 1 {
		return errors.New("exactly one source file is required")
	}
	o.MasterConfigFile = args[0]
	return nil
}

// Run runs the master config validation and returns the result of the validation as a boolean as well as any errors
// that occurred trying to validate the file
func (o *ValidateMasterConfigOptions) Run() (bool, error) {
	masterConfig, err := configapilatest.ReadAndResolveMasterConfig(o.MasterConfigFile)
	if err != nil {
		return true, err
	}

	results := validation.ValidateMasterConfig(masterConfig, nil)
	writer := tabwriter.NewWriter(o.Out, minColumnWidth, tabWidth, padding, padchar, flags)
	err = prettyPrintValidationResults(results, writer)
	if err != nil {
		return len(results.Errors) == 0, fmt.Errorf("could not print results: %v", err)
	}
	writer.Flush()
	return len(results.Errors) == 0, nil
}

const (
	minColumnWidth            = 4
	tabWidth                  = 4
	padding                   = 2
	padchar                   = byte(' ')
	flags                     = 0
	validationErrorHeadings   = "ERROR\tFIELD\tVALUE\tDETAILS\n"
	validationWarningHeadings = "WARNING\tFIELD\tVALUE\tDETAILS\n"
)

// prettyPrintValidationResults prints the contents of the ValidationResults into the buffer of a tabwriter.Writer.
// The writer must be Flush()ed after calling this to write the buffered data.
func prettyPrintValidationResults(results validation.ValidationResults, writer *tabwriter.Writer) error {
	if len(results.Errors) > 0 {
		fmt.Fprintf(writer, "VALIDATION ERRORS:\t\t\t\n")
		err := prettyPrintValidationErrorList(validationErrorHeadings, results.Errors, writer)
		if err != nil {
			return err
		}
	}
	if len(results.Warnings) > 0 {
		fmt.Fprintf(writer, "VALIDATION WARNINGS:\t\t\t\n")
		err := prettyPrintValidationErrorList(validationWarningHeadings, results.Warnings, writer)
		if err != nil {
			return err
		}
	}
	return nil
}

// prettyPrintValidationErrorList prints the contents of the ValidationErrorList into the buffer of a tabwriter.Writer.
// The writer must be Flush()ed after calling this to write the buffered data.
func prettyPrintValidationErrorList(headings string, validationErrors field.ErrorList, writer *tabwriter.Writer) error {
	if len(validationErrors) > 0 {
		fmt.Fprintf(writer, headings)
		for _, err := range validationErrors {
			err := prettyPrintValidationError(err, writer)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// prettyPrintValidationError prints the contents of the ValidationError into the buffer of a tabwriter.Writer.
// The writer must be Flush()ed after calling this to write the buffered data.
func prettyPrintValidationError(validationError *field.Error, writer *tabwriter.Writer) error {
	_, printError := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n",
		toString(validationError.Type),
		validationError.Field,
		toString(validationError.BadValue),
		validationError.Detail)

	return printError
}

const missingValue = "<none>"

func toString(v interface{}) string {
	value := fmt.Sprintf("%v", v)
	if len(value) == 0 {
		value = missingValue
	}
	return value
}

// prettyPrintGenericError prints the contents of the generic error into the buffer of a tabwriter.Writer.
// The writer must be Flush()ed after calling this to write the buffered data.
func prettyPrintGenericError(err error, writer *tabwriter.Writer) error {
	_, printError := fmt.Fprintf(writer, "\t\t\t%s\n", err.Error())
	return printError
}
