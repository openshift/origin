package admin

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/experimental/buildchain"
	"github.com/openshift/origin/pkg/cmd/experimental/config"
	"github.com/openshift/origin/pkg/cmd/experimental/policy"
	"github.com/openshift/origin/pkg/cmd/experimental/project"
	exregistry "github.com/openshift/origin/pkg/cmd/experimental/registry"
	exrouter "github.com/openshift/origin/pkg/cmd/experimental/router"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/version"
)

const longDesc = `
OpenShift Administrative Commands

Commands for managing an OpenShift cluster are exposed here. Many administrative
actions involve interaction with the OpenShift client as well.

Note: This is a beta release of OpenShift and may change significantly.  See
    https://github.com/openshift/origin for the latest information on OpenShift.
`

func NewCommandAdmin(name, fullName string, out io.Writer) *cobra.Command {
	// Main command
	cmd := &cobra.Command{
		Use:   name,
		Short: "tools for managing an OpenShift cluster",
		Long:  fmt.Sprintf(longDesc),
		Run: func(c *cobra.Command, args []string) {
			c.SetOutput(out)
			c.Help()
		},
	}

	f := clientcmd.New(cmd.PersistentFlags())

	templates.UseAdminTemplates(cmd)

	cmd.AddCommand(project.NewCmdNewProject(f, fullName, "new-project"))
	cmd.AddCommand(policy.NewCommandPolicy(f, fullName, "policy"))
	cmd.AddCommand(exrouter.NewCmdRouter(f, fullName, "router", out))
	cmd.AddCommand(exregistry.NewCmdRegistry(f, fullName, "registry", out))
	cmd.AddCommand(buildchain.NewCmdBuildChain(f, fullName, "build-chain"))
	cmd.AddCommand(config.NewCmdConfig(fullName, "config"))

	// TODO: these probably belong in a sub command
	cmd.AddCommand(admin.NewCommandCreateKubeConfig(admin.CreateKubeConfigCommandName, fullName+" "+admin.CreateKubeConfigCommandName, out))
	cmd.AddCommand(admin.NewCommandCreateBootstrapPolicyFile(admin.CreateBootstrapPolicyFileCommand, fullName+" "+admin.CreateBootstrapPolicyFileCommand, out))
	cmd.AddCommand(admin.NewCommandOverwriteBootstrapPolicy(admin.OverwriteBootstrapPolicyCommandName, fullName+" "+admin.OverwriteBootstrapPolicyCommandName, fullName+" "+admin.CreateBootstrapPolicyFileCommand, out))
	cmd.AddCommand(admin.NewCommandNodeConfig(admin.NodeConfigCommandName, fullName+" "+admin.NodeConfigCommandName, out))
	// TODO: these should be rolled up together
	cmd.AddCommand(admin.NewCommandCreateMasterCerts(admin.CreateMasterCertsCommandName, fullName+" "+admin.CreateMasterCertsCommandName, out))
	cmd.AddCommand(admin.NewCommandCreateClient(admin.CreateClientCommandName, fullName+" "+admin.CreateClientCommandName, out))
	cmd.AddCommand(admin.NewCommandCreateServerCert(admin.CreateServerCertCommandName, fullName+" "+admin.CreateServerCertCommandName, out))
	cmd.AddCommand(admin.NewCommandCreateSignerCert(admin.CreateSignerCertCommandName, fullName+" "+admin.CreateSignerCertCommandName, out))

	if name == fullName {
		cmd.AddCommand(version.NewVersionCommand(fullName))
	}

	return cmd
}
