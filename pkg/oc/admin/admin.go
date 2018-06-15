package admin

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kubecmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	ktemplates "k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/admin/cert"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics"
	"github.com/openshift/origin/pkg/oc/admin/groups"
	"github.com/openshift/origin/pkg/oc/admin/image"
	"github.com/openshift/origin/pkg/oc/admin/migrate"
	migrateetcd "github.com/openshift/origin/pkg/oc/admin/migrate/etcd"
	migrateimages "github.com/openshift/origin/pkg/oc/admin/migrate/images"
	migratehpa "github.com/openshift/origin/pkg/oc/admin/migrate/legacyhpa"
	migratestorage "github.com/openshift/origin/pkg/oc/admin/migrate/storage"
	migratetemplateinstances "github.com/openshift/origin/pkg/oc/admin/migrate/templateinstances"
	"github.com/openshift/origin/pkg/oc/admin/network"
	"github.com/openshift/origin/pkg/oc/admin/node"
	"github.com/openshift/origin/pkg/oc/admin/policy"
	"github.com/openshift/origin/pkg/oc/admin/project"
	"github.com/openshift/origin/pkg/oc/admin/prune"
	"github.com/openshift/origin/pkg/oc/admin/registry"
	"github.com/openshift/origin/pkg/oc/admin/router"
	"github.com/openshift/origin/pkg/oc/admin/top"
	"github.com/openshift/origin/pkg/oc/cli/cmd"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	"github.com/openshift/origin/pkg/oc/experimental/buildchain"
	exipfailover "github.com/openshift/origin/pkg/oc/experimental/ipfailover"
)

var adminLong = ktemplates.LongDesc(`
	Administrative Commands

	Commands for managing a cluster are exposed here. Many administrative
	actions involve interaction with the command-line client as well.`)

func NewCommandAdmin(name, fullName string, in io.Reader, out io.Writer, errout io.Writer) *cobra.Command {
	// Main command
	cmds := &cobra.Command{
		Use:   name,
		Short: "Tools for managing a cluster",
		Long:  fmt.Sprintf(adminLong),
		Run:   kcmdutil.DefaultSubCommandRun(out),
	}

	f := clientcmd.New(cmds.PersistentFlags())

	groups := ktemplates.CommandGroups{
		{
			Message: "Component Installation:",
			Commands: []*cobra.Command{
				router.NewCmdRouter(f, fullName, "router", out, errout),
				exipfailover.NewCmdIPFailoverConfig(f, fullName, "ipfailover", out, errout),
				registry.NewCmdRegistry(f, fullName, "registry", out, errout),
			},
		},
		{
			Message: "Security and Policy:",
			Commands: []*cobra.Command{
				project.NewCmdNewProject(project.NewProjectRecommendedName, fullName+" "+project.NewProjectRecommendedName, f, out),
				policy.NewCmdPolicy(policy.PolicyRecommendedName, fullName+" "+policy.PolicyRecommendedName, f, out, errout),
				groups.NewCmdGroups(groups.GroupsRecommendedName, fullName+" "+groups.GroupsRecommendedName, f, out, errout),
				cert.NewCmdCert(cert.CertRecommendedName, fullName+" "+cert.CertRecommendedName, out, errout),
				kubecmd.NewCmdCertificate(f, out),
			},
		},
		{
			Message: "Node Management:",
			Commands: []*cobra.Command{
				admin.NewCommandNodeConfig(admin.NodeConfigCommandName, fullName+" "+admin.NodeConfigCommandName, out),
				node.NewCommandManageNode(f, node.ManageNodeCommandName, fullName+" "+node.ManageNodeCommandName, out, errout),
				cmdutil.ReplaceCommandName("kubectl", fullName, ktemplates.Normalize(kubecmd.NewCmdCordon(f, out))),
				cmdutil.ReplaceCommandName("kubectl", fullName, ktemplates.Normalize(kubecmd.NewCmdUncordon(f, out))),
				cmdutil.ReplaceCommandName("kubectl", fullName, kubecmd.NewCmdDrain(f, out, errout)),
				cmdutil.ReplaceCommandName("kubectl", fullName, ktemplates.Normalize(kubecmd.NewCmdTaint(f, out))),
				network.NewCmdPodNetwork(network.PodNetworkCommandName, fullName+" "+network.PodNetworkCommandName, f, out, errout),
			},
		},
		{
			Message: "Maintenance:",
			Commands: []*cobra.Command{
				diagnostics.NewCmdDiagnostics(diagnostics.DiagnosticsRecommendedName, fullName+" "+diagnostics.DiagnosticsRecommendedName, out),
				prune.NewCommandPrune(prune.PruneRecommendedName, fullName+" "+prune.PruneRecommendedName, f, out, errout),
				buildchain.NewCmdBuildChain(name, fullName+" "+buildchain.BuildChainRecommendedCommandName, f, out),
				migrate.NewCommandMigrate(
					migrate.MigrateRecommendedName, fullName+" "+migrate.MigrateRecommendedName, f, out, errout,
					// Migration commands
					migrateimages.NewCmdMigrateImageReferences("image-references", fullName+" "+migrate.MigrateRecommendedName+" image-references", f, in, out, errout),
					migratestorage.NewCmdMigrateAPIStorage("storage", fullName+" "+migrate.MigrateRecommendedName+" storage", f, in, out, errout),
					migrateetcd.NewCmdMigrateTTLs("etcd-ttl", fullName+" "+migrate.MigrateRecommendedName+" etcd-ttl", f, in, out, errout),
					migratehpa.NewCmdMigrateLegacyHPA("legacy-hpa", fullName+" "+migrate.MigrateRecommendedName+" legacy-hpa", f, in, out, errout),
					migratetemplateinstances.NewCmdMigrateTemplateInstances("template-instances", fullName+" "+migrate.MigrateRecommendedName+" template-instances", f, in, out, errout),
				),
				top.NewCommandTop(top.TopRecommendedName, fullName+" "+top.TopRecommendedName, f, out, errout),
				image.NewCmdVerifyImageSignature(name, fullName+" "+image.VerifyRecommendedName, f, out, errout),
			},
		},
		{
			Message: "Configuration:",
			Commands: []*cobra.Command{
				admin.NewCommandCreateKubeConfig(admin.CreateKubeConfigCommandName, fullName+" "+admin.CreateKubeConfigCommandName, out),
				admin.NewCommandCreateClient(admin.CreateClientCommandName, fullName+" "+admin.CreateClientCommandName, out),

				NewCommandCreateBootstrapProjectTemplate(f, CreateBootstrapProjectTemplateCommand, fullName+" "+CreateBootstrapProjectTemplateCommand, out),
				admin.NewCommandCreateBootstrapPolicyFile(admin.CreateBootstrapPolicyFileCommand, fullName+" "+admin.CreateBootstrapPolicyFileCommand, out),

				NewCommandCreateLoginTemplate(f, CreateLoginTemplateCommand, fullName+" "+CreateLoginTemplateCommand, out),
				NewCommandCreateProviderSelectionTemplate(f, CreateProviderSelectionTemplateCommand, fullName+" "+CreateProviderSelectionTemplateCommand, out),
				NewCommandCreateErrorTemplate(f, CreateErrorTemplateCommand, fullName+" "+CreateErrorTemplateCommand, out),
			},
		},
	}

	groups.Add(cmds)
	templates.ActsAsRootCommand(cmds, []string{"options"}, groups...)

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

	cmds.AddCommand(
		// part of every root command
		cmd.NewCmdConfig(fullName, "config", f, out, errout),
		cmd.NewCmdCompletion(fullName, out),

		// hidden
		cmd.NewCmdOptions(out),
	)

	if name == fullName {
		cmds.AddCommand(cmd.NewCmdVersion(fullName, f, out, cmd.VersionOptions{}))
	}

	return cmds
}
