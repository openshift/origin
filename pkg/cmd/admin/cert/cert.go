package cert

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/util"
)

const CertRecommendedName = "ca"

// NewCmdCert implements the OpenShift cli ca command
func NewCmdCert(name, fullName string, out io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Manage certificates and keys",
		Long:  `Manage certificates and keys`,
		Run:   util.DefaultSubCommandRun(out),
	}

	cmds.AddCommand(admin.NewCommandCreateMasterCerts(admin.CreateMasterCertsCommandName, fullName+" "+admin.CreateMasterCertsCommandName, out))
	cmds.AddCommand(admin.NewCommandCreateKeyPair(admin.CreateKeyPairCommandName, fullName+" "+admin.CreateKeyPairCommandName, out))
	cmds.AddCommand(admin.NewCommandCreateServerCert(admin.CreateServerCertCommandName, fullName+" "+admin.CreateServerCertCommandName, out))
	cmds.AddCommand(admin.NewCommandCreateSignerCert(admin.CreateSignerCertCommandName, fullName+" "+admin.CreateSignerCertCommandName, out))

	return cmds
}
