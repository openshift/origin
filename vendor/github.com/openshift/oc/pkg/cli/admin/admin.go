package admin

import (
	"fmt"

	"k8s.io/kubernetes/pkg/kubectl/cmd/certificates"
	"k8s.io/kubernetes/pkg/kubectl/cmd/taint"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/cmd/drain"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	ktemplates "k8s.io/kubernetes/pkg/kubectl/util/templates"

	"github.com/openshift/oc/pkg/cli/admin/buildchain"
	"github.com/openshift/oc/pkg/cli/admin/cert"
	"github.com/openshift/oc/pkg/cli/admin/createbootstrapprojecttemplate"
	"github.com/openshift/oc/pkg/cli/admin/createerrortemplate"
	"github.com/openshift/oc/pkg/cli/admin/createkubeconfig"
	"github.com/openshift/oc/pkg/cli/admin/createlogintemplate"
	"github.com/openshift/oc/pkg/cli/admin/createproviderselectiontemplate"
	"github.com/openshift/oc/pkg/cli/admin/groups"
	"github.com/openshift/oc/pkg/cli/admin/migrate"
	migrateetcd "github.com/openshift/oc/pkg/cli/admin/migrate/etcd"
	migrateimages "github.com/openshift/oc/pkg/cli/admin/migrate/images"
	migratehpa "github.com/openshift/oc/pkg/cli/admin/migrate/legacyhpa"
	migratestorage "github.com/openshift/oc/pkg/cli/admin/migrate/storage"
	migratetemplateinstances "github.com/openshift/oc/pkg/cli/admin/migrate/templateinstances"
	"github.com/openshift/oc/pkg/cli/admin/mustgather"
	"github.com/openshift/oc/pkg/cli/admin/network"
	"github.com/openshift/oc/pkg/cli/admin/node"
	"github.com/openshift/oc/pkg/cli/admin/policy"
	"github.com/openshift/oc/pkg/cli/admin/project"
	"github.com/openshift/oc/pkg/cli/admin/prune"
	"github.com/openshift/oc/pkg/cli/admin/release"
	"github.com/openshift/oc/pkg/cli/admin/top"
	"github.com/openshift/oc/pkg/cli/admin/upgrade"
	"github.com/openshift/oc/pkg/cli/admin/verifyimagesignature"
	"github.com/openshift/oc/pkg/cli/kubectlwrappers"
	"github.com/openshift/oc/pkg/cli/options"
	cmdutil "github.com/openshift/oc/pkg/helpers/cmd"
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
				mustgather.NewMustGatherCommand(f, streams),
			},
		},
		{
			Message: "Node Management:",
			Commands: []*cobra.Command{
				cmdutil.ReplaceCommandName("kubectl", fullName, drain.NewCmdDrain(f, streams)),
				cmdutil.ReplaceCommandName("kubectl", fullName, ktemplates.Normalize(drain.NewCmdCordon(f, streams))),
				cmdutil.ReplaceCommandName("kubectl", fullName, ktemplates.Normalize(drain.NewCmdUncordon(f, streams))),
				cmdutil.ReplaceCommandName("kubectl", fullName, ktemplates.Normalize(taint.NewCmdTaint(f, streams))),
				node.NewCmdLogs(fullName, f, streams),
			},
		},
		{
			Message: "Security and Policy:",
			Commands: []*cobra.Command{
				project.NewCmdNewProject(project.NewProjectRecommendedName, fullName+" "+project.NewProjectRecommendedName, f, streams),
				policy.NewCmdPolicy(policy.PolicyRecommendedName, fullName+" "+policy.PolicyRecommendedName, f, streams),
				groups.NewCmdGroups(groups.GroupsRecommendedName, fullName+" "+groups.GroupsRecommendedName, f, streams),
				withShortDescription(certificates.NewCmdCertificate(f, streams), "Approve or reject certificate requests"),
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
				createkubeconfig.NewCommandCreateKubeConfig(createkubeconfig.CreateKubeConfigCommandName, fullName+" "+createkubeconfig.CreateKubeConfigCommandName, streams),

				createbootstrapprojecttemplate.NewCommandCreateBootstrapProjectTemplate(f, createbootstrapprojecttemplate.CreateBootstrapProjectTemplateCommand, fullName+" "+createbootstrapprojecttemplate.CreateBootstrapProjectTemplateCommand, streams),

				createlogintemplate.NewCommandCreateLoginTemplate(f, createlogintemplate.CreateLoginTemplateCommand, fullName+" "+createlogintemplate.CreateLoginTemplateCommand, streams),
				createproviderselectiontemplate.NewCommandCreateProviderSelectionTemplate(f, createproviderselectiontemplate.CreateProviderSelectionTemplateCommand, fullName+" "+createproviderselectiontemplate.CreateProviderSelectionTemplateCommand, streams),
				createerrortemplate.NewCommandCreateErrorTemplate(f, createerrortemplate.CreateErrorTemplateCommand, fullName+" "+createerrortemplate.CreateErrorTemplateCommand, streams),
			},
		},
	}

	cmds.AddCommand(cert.NewCmdCert(cert.CertRecommendedName, fullName+" "+cert.CertRecommendedName, streams))

	groups.Add(cmds)
	cmdutil.ActsAsRootCommand(cmds, []string{"options"}, groups...)

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

	return cmds
}

func withShortDescription(cmd *cobra.Command, desc string) *cobra.Command {
	cmd.Short = desc
	return cmd
}
