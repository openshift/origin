package admin

import (
	"fmt"
	"os"

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

func NewCommandAdmin(name, fullName string) *cobra.Command {
	// Main command
	cmd := &cobra.Command{
		Use:   name,
		Short: "tools for managing an OpenShift cluster",
		Long:  fmt.Sprintf(longDesc),
		Run: func(c *cobra.Command, args []string) {
			c.SetOutput(os.Stdout)
			c.Help()
		},
	}

	f := clientcmd.New(cmd.PersistentFlags())
	//in := os.Stdin
	out := os.Stdout

	templates.UseAdminTemplates(cmd)

	cmd.AddCommand(project.NewCmdNewProject(f, fullName, "new-project"))
	cmd.AddCommand(policy.NewCommandPolicy(f, fullName, "policy"))
	cmd.AddCommand(exrouter.NewCmdRouter(f, fullName, "router", out))
	cmd.AddCommand(exregistry.NewCmdRegistry(f, fullName, "registry", out))
	cmd.AddCommand(buildchain.NewCmdBuildChain(f, fullName, "build-chain"))
	cmd.AddCommand(config.NewCmdConfig(fullName, "config"))

	// TODO: these probably belong in a sub command
	cmd.AddCommand(admin.NewCommandCreateKubeConfig())
	cmd.AddCommand(admin.NewCommandCreateBootstrapPolicyFile())
	cmd.AddCommand(admin.NewCommandOverwriteBootstrapPolicy(out))
	cmd.AddCommand(admin.NewCommandNodeConfig())
	// TODO: these should be rolled up together
	cmd.AddCommand(admin.NewCommandCreateAllCerts())
	cmd.AddCommand(admin.NewCommandCreateClientCert())
	cmd.AddCommand(admin.NewCommandCreateNodeClientCert())
	cmd.AddCommand(admin.NewCommandCreateServerCert())
	cmd.AddCommand(admin.NewCommandCreateSignerCert())
	cmd.AddCommand(admin.NewCommandCreateClient())

	if name == fullName {
		cmd.AddCommand(version.NewVersionCommand(fullName))
	}

	return cmd
}
