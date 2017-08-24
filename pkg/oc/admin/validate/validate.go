package validate

import (
	"io"

	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

const (
	ValidateRecommendedName = "validate"

	validateDeprecationMessage = `and will be removed. Use "oc adm diagnostics" to run configuration validations instead.
See sub-command help text for specific instructions with "oc adm diagnostics".`
)

var validateLong = templates.LongDesc(`
	Validate configuration file integrity

	The commands here allow administrators to validate the integrity of configuration files.`)

func NewCommandValidate(name, fullName string, out, errOut io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:        name,
		Short:      "Validate configuration file integrity",
		Long:       validateLong,
		Deprecated: validateDeprecationMessage,
		Run:        cmdutil.DefaultSubCommandRun(errOut),
	}

	cmds.AddCommand(NewCommandValidateMasterConfig(ValidateMasterConfigRecommendedName,
		fullName+" "+ValidateMasterConfigRecommendedName, out))
	cmds.AddCommand(NewCommandValidateNodeConfig(ValidateNodeConfigRecommendedName,
		fullName+" "+ValidateNodeConfigRecommendedName, out))
	return cmds
}
