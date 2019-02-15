package admin

import (
	"fmt"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubecmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	ktemplates "k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/cli/admin/buildchain"
	"github.com/openshift/origin/pkg/oc/cli/admin/cert"
	"github.com/openshift/origin/pkg/oc/cli/admin/createbootstrapprojecttemplate"
	"github.com/openshift/origin/pkg/oc/cli/admin/createerrortemplate"
	"github.com/openshift/origin/pkg/oc/cli/admin/createlogintemplate"
	"github.com/openshift/origin/pkg/oc/cli/admin/createproviderselectiontemplate"
	"github.com/openshift/origin/pkg/oc/cli/admin/groups"
	"github.com/openshift/origin/pkg/oc/cli/admin/migrate"
	migrateetcd "github.com/openshift/origin/pkg/oc/cli/admin/migrate/etcd"
	migrateimages "github.com/openshift/origin/pkg/oc/cli/admin/migrate/images"
	migratehpa "github.com/openshift/origin/pkg/oc/cli/admin/migrate/legacyhpa"
	migratestorage "github.com/openshift/origin/pkg/oc/cli/admin/migrate/storage"
	migratetemplateinstances "github.com/openshift/origin/pkg/oc/cli/admin/migrate/templateinstances"
	"github.com/openshift/origin/pkg/oc/cli/admin/network"
	"github.com/openshift/origin/pkg/oc/cli/admin/node"
	"github.com/openshift/origin/pkg/oc/cli/admin/node/logs"
	"github.com/openshift/origin/pkg/oc/cli/admin/policy"
	"github.com/openshift/origin/pkg/oc/cli/admin/project"
	"github.com/openshift/origin/pkg/oc/cli/admin/prune"
	"github.com/openshift/origin/pkg/oc/cli/admin/release"
	"github.com/openshift/origin/pkg/oc/cli/admin/top"
	"github.com/openshift/origin/pkg/oc/cli/admin/upgrade"
	"github.com/openshift/origin/pkg/oc/cli/admin/verifyimagesignature"
	"github.com/openshift/origin/pkg/oc/cli/kubectlwrappers"
	"github.com/openshift/origin/pkg/oc/cli/options"
	"github.com/openshift/origin/pkg/oc/cli/version"
)

var adminLong = ktemplates.LongDesc(`
	Administrative Commands

	Actions for administering an OpenShift cluster are exposed here.`)

func NewCommandAdmin(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	// Main command
	cmds := &cobra.Command{
		Use:   name,
		Short: "Tools for managing a cluster",
		Long:  fmt.Sprintf(adminLong),
		Run:   kcmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	groups := ktemplates.CommandGroups{
		{
			Message: "Cluster Management:",
			Commands: []*cobra.Command{
				upgrade.New(f, fullName, streams),
				top.NewCommandTop(top.TopRecommendedName, fullName+" "+top.TopRecommendedName, f, streams),
			},
		},
		{
			Message: "Node Management:",
			Commands: []*cobra.Command{
				cmdutil.ReplaceCommandName("kubectl", fullName, kubecmd.NewCmdDrain(f, streams)),
				cmdutil.ReplaceCommandName("kubectl", fullName, ktemplates.Normalize(kubecmd.NewCmdCordon(f, streams))),
				cmdutil.ReplaceCommandName("kubectl", fullName, ktemplates.Normalize(kubecmd.NewCmdUncordon(f, streams))),
				cmdutil.ReplaceCommandName("kubectl", fullName, ktemplates.Normalize(kubecmd.NewCmdTaint(f, streams))),
				logs.NewCmdLogs(fullName, f, streams),
			},
		},
		{
			Message: "Security and Policy:",
			Commands: []*cobra.Command{
				project.NewCmdNewProject(project.NewProjectRecommendedName, fullName+" "+project.NewProjectRecommendedName, f, streams),
				policy.NewCmdPolicy(policy.PolicyRecommendedName, fullName+" "+policy.PolicyRecommendedName, f, streams),
				groups.NewCmdGroups(groups.GroupsRecommendedName, fullName+" "+groups.GroupsRecommendedName, f, streams),
				withShortDescription(kubecmd.NewCmdCertificate(f, streams), "Approve or reject certificate requests"),
				network.NewCmdPodNetwork(network.PodNetworkCommandName, fullName+" "+network.PodNetworkCommandName, f, streams),
			},
		},
		{
			Message: "Maintenance:",
			Commands: []*cobra.Command{
				prune.NewCommandPrune(prune.PruneRecommendedName, fullName+" "+prune.PruneRecommendedName, f, streams),
				migrate.NewCommandMigrate(
					migrate.MigrateRecommendedName, fullName+" "+migrate.MigrateRecommendedName, f, streams,
					// Migration commands
					migrateimages.NewCmdMigrateImageReferences("image-references", fullName+" "+migrate.MigrateRecommendedName+" image-references", f, streams),
					migratestorage.NewCmdMigrateAPIStorage("storage", fullName+" "+migrate.MigrateRecommendedName+" storage", f, streams),
					migrateetcd.NewCmdMigrateTTLs("etcd-ttl", fullName+" "+migrate.MigrateRecommendedName+" etcd-ttl", f, streams),
					migratehpa.NewCmdMigrateLegacyHPA("legacy-hpa", fullName+" "+migrate.MigrateRecommendedName+" legacy-hpa", f, streams),
					migratetemplateinstances.NewCmdMigrateTemplateInstances("template-instances", fullName+" "+migrate.MigrateRecommendedName+" template-instances", f, streams),
				),
			},
		},
		{
			Message: "Configuration:",
			Commands: []*cobra.Command{
				cert.NewCmdCert(cert.CertRecommendedName, fullName+" "+cert.CertRecommendedName, streams),
				admin.NewCommandCreateKubeConfig(admin.CreateKubeConfigCommandName, fullName+" "+admin.CreateKubeConfigCommandName, streams),
				admin.NewCommandCreateClient(admin.CreateClientCommandName, fullName+" "+admin.CreateClientCommandName, streams),

				createbootstrapprojecttemplate.NewCommandCreateBootstrapProjectTemplate(f, createbootstrapprojecttemplate.CreateBootstrapProjectTemplateCommand, fullName+" "+createbootstrapprojecttemplate.CreateBootstrapProjectTemplateCommand, streams),
				admin.NewCommandCreateBootstrapPolicyFile(admin.CreateBootstrapPolicyFileCommand, fullName+" "+admin.CreateBootstrapPolicyFileCommand, streams),

				createlogintemplate.NewCommandCreateLoginTemplate(f, createlogintemplate.CreateLoginTemplateCommand, fullName+" "+createlogintemplate.CreateLoginTemplateCommand, streams),
				createproviderselectiontemplate.NewCommandCreateProviderSelectionTemplate(f, createproviderselectiontemplate.CreateProviderSelectionTemplateCommand, fullName+" "+createproviderselectiontemplate.CreateProviderSelectionTemplateCommand, streams),
				createerrortemplate.NewCommandCreateErrorTemplate(f, createerrortemplate.CreateErrorTemplateCommand, fullName+" "+createerrortemplate.CreateErrorTemplateCommand, streams),
			},
		},
	}

	groups.Add(cmds)
	templates.ActsAsRootCommand(cmds, []string{"options"}, groups...)

	// Deprecated commands that are bundled with the binary but not displayed to end users directly
	deprecatedCommands := []*cobra.Command{
		// these will be removed soon
		node.NewCommandManageNode(f, node.ManageNodeCommandName, fullName+" "+node.ManageNodeCommandName, streams),
	}
	deprecatedCACommands := []*cobra.Command{
		admin.NewCommandCreateMasterCerts(admin.CreateMasterCertsCommandName, fullName+" "+admin.CreateMasterCertsCommandName, streams),
		admin.NewCommandCreateKeyPair(admin.CreateKeyPairCommandName, fullName+" "+admin.CreateKeyPairCommandName, streams),
		admin.NewCommandCreateServerCert(admin.CreateServerCertCommandName, fullName+" "+admin.CreateServerCertCommandName, streams),
		admin.NewCommandCreateSignerCert(admin.CreateSignerCertCommandName, fullName+" "+admin.CreateSignerCertCommandName, streams),
	}
	for _, cmd := range deprecatedCACommands {
		// Unsetting Short description will not show this command in help
		cmd.Short = ""
		cmd.Deprecated = fmt.Sprintf("Use '%s ca' instead.", fullName)
		cmds.AddCommand(cmd)
	}
	for _, cmd := range deprecatedCommands {
		// Unsetting Short description will not show this command in help
		cmd.Short = ""
		cmd.Deprecated = fmt.Sprintf("'%s %s' is DEPRECATED and will be removed in a future version.", fullName, cmd.Name())
		cmds.AddCommand(cmd)
	}

	cmds.AddCommand(
		release.NewCmd(f, fullName, streams),
		buildchain.NewCmdBuildChain(name, fullName+" "+buildchain.BuildChainRecommendedCommandName, f, streams),
		verifyimagesignature.NewCmdVerifyImageSignature(name, fullName+" "+verifyimagesignature.VerifyRecommendedName, f, streams),

		// part of every root command
		kubectlwrappers.NewCmdConfig(fullName, "config", f, streams),
		kubectlwrappers.NewCmdCompletion(fullName, streams),

		// hidden
		options.NewCmdOptions(streams),
	)

	if name == fullName {
		cmds.AddCommand(version.NewCmdVersion(fullName, f, version.NewVersionOptions(false, streams)))
	}

	return cmds
}

func withShortDescription(cmd *cobra.Command, desc string) *cobra.Command {
	cmd.Short = desc
	return cmd
}
