package validate

import (
	"io"

	"github.com/spf13/cobra"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

const (
	ValidateRecommendedName = "validate"

	validateLong = `Validate configuration file integrity

The commands here allow administrators to validate the integrity of configuration files.
`

	validateDeprecationMessage = `The commands here are deprecated and will be removed. Use 'openshift ex diagnostics' to run configuration validations instead.
See sub-command help text for specific instructions with 'openshift ex diagnostics'.`
)

func NewCommandValidate(name, fullName string, out io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:        name,
		Short:      "Validate configuration file integrity",
		Long:       validateLong,
		Deprecated: validateDeprecationMessage,
		Run:        cmdutil.DefaultSubCommandRun(out),
	}

	cmds.AddCommand(NewCommandValidateMasterConfig(ValidateMasterConfigRecommendedName,
		fullName+" "+ValidateMasterConfigRecommendedName, out))
	cmds.AddCommand(NewCommandValidateNodeConfig(ValidateNodeConfigRecommendedName,
		fullName+" "+ValidateNodeConfigRecommendedName, out))
	return cmds
}
