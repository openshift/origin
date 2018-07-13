package admin

import (
	"fmt"

	"github.com/spf13/cobra"

	kubecmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	ktemplates "k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

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
	"github.com/openshift/origin/pkg/oc/experimental/buildchain"
	exipfailover "github.com/openshift/origin/pkg/oc/experimental/ipfailover"
)

var adminLong = ktemplates.LongDesc(`
	Administrative Commands

	Commands for managing a cluster are exposed here. Many administrative
	actions involve interaction with the command-line client as well.`)

func NewCommandAdmin(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	// Main command
	cmds := &cobra.Command{
		Use:   name,
		Short: "Tools for managing a cluster",
		Long:  fmt.Sprintf(adminLong),
		Run:   kcmdutil.DefaultSubCommandRun(streams.Out),
	}

	groups := ktemplates.CommandGroups{
		{
			Message: "Component Installation:",
			Commands: []*cobra.Command{
				router.NewCmdRouter(f, fullName, "router", streams.Out, streams.ErrOut),
				exipfailover.NewCmdIPFailoverConfig(f, fullName, "ipfailover", streams),
				registry.NewCmdRegistry(f, fullName, "registry", streams.Out, streams.ErrOut),
			},
		},
		{
			Message: "Security and Policy:",
			Commands: []*cobra.Command{
				project.NewCmdNewProject(project.NewProjectRecommendedName, fullName+" "+project.NewProjectRecommendedName, f, streams.Out),
				policy.NewCmdPolicy(policy.PolicyRecommendedName, fullName+" "+policy.PolicyRecommendedName, f, streams.Out, streams.ErrOut),
				groups.NewCmdGroups(groups.GroupsRecommendedName, fullName+" "+groups.GroupsRecommendedName, f, streams),
				cert.NewCmdCert(cert.CertRecommendedName, fullName+" "+cert.CertRecommendedName, streams.Out, streams.ErrOut),
				kubecmd.NewCmdCertificate(f, streams),
			},
		},
		{
			Message: "Node Management:",
			Commands: []*cobra.Command{
				admin.NewCommandNodeConfig(admin.NodeConfigCommandName, fullName+" "+admin.NodeConfigCommandName, streams.Out),
				node.NewCommandManageNode(f, node.ManageNodeCommandName, fullName+" "+node.ManageNodeCommandName, streams.Out, streams.ErrOut),
				cmdutil.ReplaceCommandName("kubectl", fullName, ktemplates.Normalize(kubecmd.NewCmdCordon(f, streams))),
				cmdutil.ReplaceCommandName("kubectl", fullName, ktemplates.Normalize(kubecmd.NewCmdUncordon(f, streams))),
				cmdutil.ReplaceCommandName("kubectl", fullName, kubecmd.NewCmdDrain(f, streams)),
				cmdutil.ReplaceCommandName("kubectl", fullName, ktemplates.Normalize(kubecmd.NewCmdTaint(f, streams))),
				network.NewCmdPodNetwork(network.PodNetworkCommandName, fullName+" "+network.PodNetworkCommandName, f, streams.Out, streams.ErrOut),
			},
		},
		{
			Message: "Maintenance:",
			Commands: []*cobra.Command{
				diagnostics.NewCmdDiagnostics(diagnostics.DiagnosticsRecommendedName, fullName+" "+diagnostics.DiagnosticsRecommendedName, f, streams),
				prune.NewCommandPrune(prune.PruneRecommendedName, fullName+" "+prune.PruneRecommendedName, f, streams),
				buildchain.NewCmdBuildChain(name, fullName+" "+buildchain.BuildChainRecommendedCommandName, f, streams),
				migrate.NewCommandMigrate(
					migrate.MigrateRecommendedName, fullName+" "+migrate.MigrateRecommendedName, f, streams.Out, streams.ErrOut,
					// Migration commands
					migrateimages.NewCmdMigrateImageReferences("image-references", fullName+" "+migrate.MigrateRecommendedName+" image-references", f, streams.In, streams.Out, streams.ErrOut),
					migratestorage.NewCmdMigrateAPIStorage("storage", fullName+" "+migrate.MigrateRecommendedName+" storage", f, streams.In, streams.Out, streams.ErrOut),
					migrateetcd.NewCmdMigrateTTLs("etcd-ttl", fullName+" "+migrate.MigrateRecommendedName+" etcd-ttl", f, streams.In, streams.Out, streams.ErrOut),
					migratehpa.NewCmdMigrateLegacyHPA("legacy-hpa", fullName+" "+migrate.MigrateRecommendedName+" legacy-hpa", f, streams.In, streams.Out, streams.ErrOut),
					migratetemplateinstances.NewCmdMigrateTemplateInstances("template-instances", fullName+" "+migrate.MigrateRecommendedName+" template-instances", f, streams.In, streams.Out, streams.ErrOut),
				),
				top.NewCommandTop(top.TopRecommendedName, fullName+" "+top.TopRecommendedName, f, streams.Out, streams.ErrOut),
				image.NewCmdVerifyImageSignature(name, fullName+" "+image.VerifyRecommendedName, f, streams.Out, streams.ErrOut),
			},
		},
		{
			Message: "Configuration:",
			Commands: []*cobra.Command{
				admin.NewCommandCreateKubeConfig(admin.CreateKubeConfigCommandName, fullName+" "+admin.CreateKubeConfigCommandName, streams.Out),
				admin.NewCommandCreateClient(admin.CreateClientCommandName, fullName+" "+admin.CreateClientCommandName, streams.Out),

				NewCommandCreateBootstrapProjectTemplate(f, CreateBootstrapProjectTemplateCommand, fullName+" "+CreateBootstrapProjectTemplateCommand, streams.Out),
				admin.NewCommandCreateBootstrapPolicyFile(admin.CreateBootstrapPolicyFileCommand, fullName+" "+admin.CreateBootstrapPolicyFileCommand, streams.Out),

				NewCommandCreateLoginTemplate(f, CreateLoginTemplateCommand, fullName+" "+CreateLoginTemplateCommand, streams.Out),
				NewCommandCreateProviderSelectionTemplate(f, CreateProviderSelectionTemplateCommand, fullName+" "+CreateProviderSelectionTemplateCommand, streams.Out),
				NewCommandCreateErrorTemplate(f, CreateErrorTemplateCommand, fullName+" "+CreateErrorTemplateCommand, streams.Out),
			},
		},
	}

	groups.Add(cmds)
	templates.ActsAsRootCommand(cmds, []string{"options"}, groups...)

	// Deprecated commands that are bundled with the binary but not displayed to end users directly
	deprecatedCommands := []*cobra.Command{
		admin.NewCommandCreateMasterCerts(admin.CreateMasterCertsCommandName, fullName+" "+admin.CreateMasterCertsCommandName, streams.Out),
		admin.NewCommandCreateKeyPair(admin.CreateKeyPairCommandName, fullName+" "+admin.CreateKeyPairCommandName, streams.Out),
		admin.NewCommandCreateServerCert(admin.CreateServerCertCommandName, fullName+" "+admin.CreateServerCertCommandName, streams.Out),
		admin.NewCommandCreateSignerCert(admin.CreateSignerCertCommandName, fullName+" "+admin.CreateSignerCertCommandName, streams.Out),
	}
	for _, cmd := range deprecatedCommands {
		// Unsetting Short description will not show this command in help
		cmd.Short = ""
		cmd.Deprecated = fmt.Sprintf("Use '%s ca' instead.", fullName)
		cmds.AddCommand(cmd)
	}

	cmds.AddCommand(
		// part of every root command
		cmd.NewCmdConfig(fullName, "config", f, streams),
		cmd.NewCmdCompletion(fullName, streams),

		// hidden
		cmd.NewCmdOptions(streams),
	)

	if name == fullName {
		cmds.AddCommand(cmd.NewCmdVersion(fullName, f, cmd.NewVersionOptions(false, streams)))
	}

	return cmds
}
