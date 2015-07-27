package admin

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/admin/cert"
	"github.com/openshift/origin/pkg/cmd/admin/node"
	"github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/cmd/admin/project"
	"github.com/openshift/origin/pkg/cmd/admin/prune"
	"github.com/openshift/origin/pkg/cmd/admin/registry"
	"github.com/openshift/origin/pkg/cmd/admin/router"
	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/experimental/buildchain"
	exipfailover "github.com/openshift/origin/pkg/cmd/experimental/ipfailover"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/version"
)

const adminLong = `
Administrative Commands

Commands for managing a cluster are exposed here. Many administrative
actions involve interaction with the command-line client as well.`

func NewCommandAdmin(name, fullName string, out io.Writer) *cobra.Command {
	// Main command
	cmds := &cobra.Command{
		Use:   name,
		Short: "Tools for managing a cluster",
		Long:  fmt.Sprintf(adminLong),
		Run:   cmdutil.DefaultSubCommandRun(out),
	}

	f := clientcmd.New(cmds.PersistentFlags())

	groups := templates.CommandGroups{
		{
			Message: "Basic Commands:",
			Commands: []*cobra.Command{
				project.NewCmdNewProject(project.NewProjectRecommendedName, fullName+" "+project.NewProjectRecommendedName, f, out),
				policy.NewCmdPolicy(policy.PolicyRecommendedName, fullName+" "+policy.PolicyRecommendedName, f, out),
			},
		},
		{
			Message: "Install Commands:",
			Commands: []*cobra.Command{
				router.NewCmdRouter(f, fullName, "router", out),
				exipfailover.NewCmdIPFailoverConfig(f, fullName, "ipfailover", out),
				registry.NewCmdRegistry(f, fullName, "registry", out),
			},
		},
		{
			Message: "Maintenance Commands:",
			Commands: []*cobra.Command{
				buildchain.NewCmdBuildChain(name, fullName+" "+buildchain.BuildChainRecommendedCommandName, f, out),
				node.NewCommandManageNode(f, node.ManageNodeCommandName, fullName+" "+node.ManageNodeCommandName, out),
				prune.NewCommandPrune(prune.PruneRecommendedName, fullName+" "+prune.PruneRecommendedName, f, out),
			},
		},
		{
			Message: "Settings Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdConfig(fullName, "config"),

				// TODO: these probably belong in a sub command
				admin.NewCommandCreateKubeConfig(admin.CreateKubeConfigCommandName, fullName+" "+admin.CreateKubeConfigCommandName, out),
				admin.NewCommandCreateClient(admin.CreateClientCommandName, fullName+" "+admin.CreateClientCommandName, out),
			},
		},
		{
			Message: "Advanced Commands:",
			Commands: []*cobra.Command{
				admin.NewCommandCreateBootstrapProjectTemplate(f, admin.CreateBootstrapProjectTemplateCommand, fullName+" "+admin.CreateBootstrapProjectTemplateCommand, out),
				admin.NewCommandCreateBootstrapPolicyFile(admin.CreateBootstrapPolicyFileCommand, fullName+" "+admin.CreateBootstrapPolicyFileCommand, out),
				admin.NewCommandOverwriteBootstrapPolicy(admin.OverwriteBootstrapPolicyCommandName, fullName+" "+admin.OverwriteBootstrapPolicyCommandName, fullName+" "+admin.CreateBootstrapPolicyFileCommand, out),
				admin.NewCommandNodeConfig(admin.NodeConfigCommandName, fullName+" "+admin.NodeConfigCommandName, out),
				cert.NewCmdCert(cert.CertRecommendedName, fullName+" "+cert.CertRecommendedName, out),
			},
		},
	}

	groups.Add(cmds)
	templates.ActsAsRootCommand(cmds, groups...)

	// Deprecated commands that are bundled with the binary but not displayed to end users directly
	deprecatedCommands := []*cobra.Command{
		admin.NewCommandCreateMasterCerts(admin.CreateMasterCertsCommandName, fullName+" "+admin.CreateMasterCertsCommandName, out),
		admin.NewCommandCreateKeyPair(admin.CreateKeyPairCommandName, fullName+" "+admin.CreateKeyPairCommandName, out),
		admin.NewCommandCreateServerCert(admin.CreateServerCertCommandName, fullName+" "+admin.CreateServerCertCommandName, out),
		admin.NewCommandCreateSignerCert(admin.CreateSignerCertCommandName, fullName+" "+admin.CreateSignerCertCommandName, out),
	}
	for _, cmd := range deprecatedCommands {
		// Unsetting Short description will not show this command in help
		cmd.Short = ""
		cmd.Deprecated = fmt.Sprintf("Use '%s ca' instead.", fullName)
		cmds.AddCommand(cmd)
	}

	if name == fullName {
		cmds.AddCommand(version.NewVersionCommand(fullName))
	}

	cmds.AddCommand(cmd.NewCmdOptions(out))

	return cmds
}
