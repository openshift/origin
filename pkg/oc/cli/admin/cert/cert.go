package cert

import (
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/server/admin"
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
		admin.NewCommandEncrypt(admin.EncryptCommandName, fullName+" "+admin.EncryptCommandName, streams),
		admin.NewCommandDecrypt(admin.DecryptCommandName, fullName+" "+admin.DecryptCommandName, fullName+" "+admin.EncryptCommandName, streams),
	}

	for _, cmd := range subCommands {
		// Unsetting Short description will not show this command in help
		cmd.Short = ""
		cmd.Deprecated = "and will be removed in the future version"
		cmds.AddCommand(cmd)
	}

	return cmds
}
