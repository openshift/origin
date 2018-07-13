package cert

import (
	"io"

	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/server/admin"
)

const CertRecommendedName = "ca"

// NewCmdCert implements the OpenShift cli ca command
func NewCmdCert(name, fullName string, out io.Writer, errout io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Manage certificates and keys",
		Long:  `Manage certificates and keys`,
		Run:   cmdutil.DefaultSubCommandRun(errout),
	}

	cmds.AddCommand(admin.NewCommandCreateMasterCerts(admin.CreateMasterCertsCommandName, fullName+" "+admin.CreateMasterCertsCommandName, out))
	cmds.AddCommand(admin.NewCommandCreateKeyPair(admin.CreateKeyPairCommandName, fullName+" "+admin.CreateKeyPairCommandName, out))
	cmds.AddCommand(admin.NewCommandCreateServerCert(admin.CreateServerCertCommandName, fullName+" "+admin.CreateServerCertCommandName, out))
	cmds.AddCommand(admin.NewCommandCreateSignerCert(admin.CreateSignerCertCommandName, fullName+" "+admin.CreateSignerCertCommandName, out))

	cmds.AddCommand(admin.NewCommandEncrypt(admin.EncryptCommandName, fullName+" "+admin.EncryptCommandName, out, errout))
	cmds.AddCommand(admin.NewCommandDecrypt(admin.DecryptCommandName, fullName+" "+admin.DecryptCommandName, fullName+" "+admin.EncryptCommandName, out))

	return cmds
}
