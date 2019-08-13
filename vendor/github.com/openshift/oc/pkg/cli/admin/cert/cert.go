package cert

import (
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

const CertRecommendedName = "ca"

// NewCmdCert implements the OpenShift cli ca command
func NewCmdCert(name, fullName string, streams genericclioptions.IOStreams) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:        name,
		Long:       "Manage certificates and keys",
		Short:      "",
		Run:        cmdutil.DefaultSubCommandRun(streams.ErrOut),
		Deprecated: "and will be removed in the future version",
		Hidden:     true,
	}

	subCommands := []*cobra.Command{
		NewCommandEncrypt(EncryptCommandName, fullName+" "+EncryptCommandName, streams),
		NewCommandDecrypt(DecryptCommandName, fullName+" "+DecryptCommandName, fullName+" "+EncryptCommandName, streams),
	}

	for _, cmd := range subCommands {
		// Unsetting Short description will not show this command in help
		cmd.Short = ""
		cmd.Deprecated = "and will be removed in the future version"
		cmds.AddCommand(cmd)
	}

	return cmds
}
