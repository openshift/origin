package cert

import (
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/cmd/server/admin"
)

const CertRecommendedName = "ca"

// NewCmdCert implements the OpenShift cli ca command
func NewCmdCert(name, fullName string, streams genericclioptions.IOStreams) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Manage certificates and keys",
		Long:  `Manage certificates and keys`,
		Run:   cmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	cmds.AddCommand(admin.NewCommandCreateMasterCerts(admin.CreateMasterCertsCommandName, fullName+" "+admin.CreateMasterCertsCommandName, streams))
	cmds.AddCommand(admin.NewCommandCreateKeyPair(admin.CreateKeyPairCommandName, fullName+" "+admin.CreateKeyPairCommandName, streams))
	cmds.AddCommand(admin.NewCommandCreateServerCert(admin.CreateServerCertCommandName, fullName+" "+admin.CreateServerCertCommandName, streams))
	cmds.AddCommand(admin.NewCommandCreateSignerCert(admin.CreateSignerCertCommandName, fullName+" "+admin.CreateSignerCertCommandName, streams))

	cmds.AddCommand(admin.NewCommandEncrypt(admin.EncryptCommandName, fullName+" "+admin.EncryptCommandName, streams))
	cmds.AddCommand(admin.NewCommandDecrypt(admin.DecryptCommandName, fullName+" "+admin.DecryptCommandName, fullName+" "+admin.EncryptCommandName, streams))

	return cmds
}
