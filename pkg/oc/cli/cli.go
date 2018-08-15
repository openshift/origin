package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	kubecmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	ktemplates "k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/infra/builder"
	"github.com/openshift/origin/pkg/cmd/infra/deployer"
	irouter "github.com/openshift/origin/pkg/cmd/infra/router"
	"github.com/openshift/origin/pkg/cmd/recycle"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/term"
	"github.com/openshift/origin/pkg/oc/cli/admin"
	"github.com/openshift/origin/pkg/oc/cli/admin/buildchain"
	"github.com/openshift/origin/pkg/oc/cli/admin/diagnostics"
	sync "github.com/openshift/origin/pkg/oc/cli/admin/groups/sync"
	exipfailover "github.com/openshift/origin/pkg/oc/cli/admin/ipfailover"
	"github.com/openshift/origin/pkg/oc/cli/buildlogs"
	"github.com/openshift/origin/pkg/oc/cli/cancelbuild"
	"github.com/openshift/origin/pkg/oc/cli/cluster"
	"github.com/openshift/origin/pkg/oc/cli/debug"
	configcmd "github.com/openshift/origin/pkg/oc/cli/experimental/config"
	"github.com/openshift/origin/pkg/oc/cli/experimental/dockergc"
	"github.com/openshift/origin/pkg/oc/cli/export"
	"github.com/openshift/origin/pkg/oc/cli/expose"
	"github.com/openshift/origin/pkg/oc/cli/extract"
	"github.com/openshift/origin/pkg/oc/cli/idle"
	"github.com/openshift/origin/pkg/oc/cli/image"
	"github.com/openshift/origin/pkg/oc/cli/importer"
	"github.com/openshift/origin/pkg/oc/cli/importimage"
	"github.com/openshift/origin/pkg/oc/cli/kubectlwrappers"
	"github.com/openshift/origin/pkg/oc/cli/login"
	"github.com/openshift/origin/pkg/oc/cli/logout"
	"github.com/openshift/origin/pkg/oc/cli/logs"
	"github.com/openshift/origin/pkg/oc/cli/newapp"
	"github.com/openshift/origin/pkg/oc/cli/newbuild"
	"github.com/openshift/origin/pkg/oc/cli/observe"
	"github.com/openshift/origin/pkg/oc/cli/options"
	"github.com/openshift/origin/pkg/oc/cli/policy"
	"github.com/openshift/origin/pkg/oc/cli/process"
	"github.com/openshift/origin/pkg/oc/cli/project"
	"github.com/openshift/origin/pkg/oc/cli/projects"
	"github.com/openshift/origin/pkg/oc/cli/registry"
	"github.com/openshift/origin/pkg/oc/cli/requestproject"
	"github.com/openshift/origin/pkg/oc/cli/rollback"
	"github.com/openshift/origin/pkg/oc/cli/rollout"
	"github.com/openshift/origin/pkg/oc/cli/rsh"
	"github.com/openshift/origin/pkg/oc/cli/rsync"
	"github.com/openshift/origin/pkg/oc/cli/secrets"
	"github.com/openshift/origin/pkg/oc/cli/serviceaccounts"
	"github.com/openshift/origin/pkg/oc/cli/set"
	"github.com/openshift/origin/pkg/oc/cli/startbuild"
	"github.com/openshift/origin/pkg/oc/cli/status"
	"github.com/openshift/origin/pkg/oc/cli/tag"
	"github.com/openshift/origin/pkg/oc/cli/types"
	"github.com/openshift/origin/pkg/oc/cli/version"
	"github.com/openshift/origin/pkg/oc/cli/whoami"
	"github.com/openshift/origin/pkg/oc/util/ocscheme"
)

const productName = `OpenShift`

var (
	cliLong = ktemplates.LongDesc(`
    ` + productName + ` Client

    This client helps you develop, build, deploy, and run your applications on any
    OpenShift or Kubernetes compatible platform. It also includes the administrative
    commands for managing a cluster under the 'adm' subcommand.`)

	cliExplain = ktemplates.LongDesc(`
    To create a new application, login to your server and then run new-app:

        %[1]s login https://mycluster.mycompany.com
        %[1]s new-app centos/ruby-25-centos7~https://github.com/sclorg/ruby-ex.git
        %[1]s logs -f bc/ruby-ex

    This will create an application based on the Docker image 'centos/ruby-25-centos7' that builds the source code from GitHub. A build will start automatically, push the resulting image to the registry, and a deployment will roll that change out in your project.

    Once your application is deployed, use the status, describe, and get commands to see more about the created components:

        %[1]s status
        %[1]s describe deploymentconfig ruby-ex
        %[1]s get pods

    To make this application visible outside of the cluster, use the expose command on the service we just created to create a 'route' (which will connect your application over the HTTP port to a public domain name).

        %[1]s expose svc/ruby-ex
        %[1]s status

    You should now see the URL the application can be reached at.

    To see the full list of commands supported, run '%[1]s --help'.`)
)

func NewCommandCLI(name, fullName string, in io.Reader, out, errout io.Writer) *cobra.Command {
	// Main command
	cmds := &cobra.Command{
		Use:   name,
		Short: "Command line tools for managing applications",
		Long:  cliLong,
		Run: func(c *cobra.Command, args []string) {
			explainOut := term.NewResponsiveWriter(out)
			c.SetOutput(explainOut)
			kcmdutil.RequireNoArguments(c, args)
			fmt.Fprintf(explainOut, "%s\n\n%s\n", cliLong, fmt.Sprintf(cliExplain, fullName))
		},
		BashCompletionFunction: bashCompletionFunc,
	}

	kubeConfigFlags := genericclioptions.NewConfigFlags()
	kubeConfigFlags.AddFlags(cmds.PersistentFlags())
	matchVersionKubeConfigFlags := kcmdutil.NewMatchVersionFlags(kubeConfigFlags)
	matchVersionKubeConfigFlags.AddFlags(cmds.PersistentFlags())
	cmds.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	f := kcmdutil.NewFactory(matchVersionKubeConfigFlags)

	ioStreams := genericclioptions.IOStreams{In: in, Out: out, ErrOut: errout}

	loginCmd := login.NewCmdLogin(fullName, f, ioStreams)
	secretcmds := secrets.NewCmdSecrets(secrets.SecretsRecommendedName, fullName+" "+secrets.SecretsRecommendedName, f, ioStreams)

	groups := ktemplates.CommandGroups{
		{
			Message: "Basic Commands:",
			Commands: []*cobra.Command{
				types.NewCmdTypes(fullName, f, ioStreams),
				loginCmd,
				requestproject.NewCmdRequestProject(fullName, f, ioStreams),
				newapp.NewCmdNewApplication(newapp.NewAppRecommendedCommandName, fullName, f, ioStreams),
				status.NewCmdStatus(status.StatusRecommendedName, fullName, fullName+" "+status.StatusRecommendedName, f, ioStreams),
				project.NewCmdProject(fullName, f, ioStreams),
				projects.NewCmdProjects(fullName, f, ioStreams),
				kubectlwrappers.NewCmdExplain(fullName, f, ioStreams),
				cluster.NewCmdCluster(cluster.ClusterRecommendedName, fullName+" "+cluster.ClusterRecommendedName, f, ioStreams),
			},
		},
		{
			Message: "Build and Deploy Commands:",
			Commands: []*cobra.Command{
				rollout.NewCmdRollout(fullName, f, ioStreams),
				rollback.NewCmdRollback(fullName, f, ioStreams),
				newbuild.NewCmdNewBuild(newbuild.NewBuildRecommendedCommandName, fullName, f, ioStreams),
				startbuild.NewCmdStartBuild(fullName, f, ioStreams),
				cancelbuild.NewCmdCancelBuild(cancelbuild.CancelBuildRecommendedCommandName, fullName, f, ioStreams),
				importimage.NewCmdImportImage(fullName, f, ioStreams),
				tag.NewCmdTag(fullName, f, ioStreams),
			},
		},
		{
			Message: "Application Management Commands:",
			Commands: []*cobra.Command{
				kubectlwrappers.NewCmdGet(fullName, f, ioStreams),
				kubectlwrappers.NewCmdDescribe(fullName, f, ioStreams),
				kubectlwrappers.NewCmdEdit(fullName, f, ioStreams),
				set.NewCmdSet(fullName, f, ioStreams),
				kubectlwrappers.NewCmdLabel(fullName, f, ioStreams),
				kubectlwrappers.NewCmdAnnotate(fullName, f, ioStreams),
				expose.NewCmdExpose(fullName, f, ioStreams),
				kubectlwrappers.NewCmdDelete(fullName, f, ioStreams),
				kubectlwrappers.NewCmdScale(fullName, f, ioStreams),
				kubectlwrappers.NewCmdAutoscale(fullName, f, ioStreams),
				secretcmds,
				serviceaccounts.NewCmdServiceAccounts(serviceaccounts.ServiceAccountsRecommendedName, fullName+" "+serviceaccounts.ServiceAccountsRecommendedName, f, ioStreams),
			},
		},
		{
			Message: "Troubleshooting and Debugging Commands:",
			Commands: []*cobra.Command{
				logs.NewCmdLogs(logs.LogsRecommendedCommandName, fullName, f, ioStreams),
				rsh.NewCmdRsh(rsh.RshRecommendedName, fullName, f, ioStreams),
				rsync.NewCmdRsync(rsync.RsyncRecommendedName, fullName, f, ioStreams),
				kubectlwrappers.NewCmdPortForward(fullName, f, ioStreams),
				debug.NewCmdDebug(fullName, f, ioStreams),
				kubectlwrappers.NewCmdExec(fullName, f, ioStreams),
				kubectlwrappers.NewCmdProxy(fullName, f, ioStreams),
				kubectlwrappers.NewCmdAttach(fullName, f, ioStreams),
				kubectlwrappers.NewCmdRun(fullName, f, ioStreams),
				kubectlwrappers.NewCmdCp(fullName, f, ioStreams),
				kubectlwrappers.NewCmdWait(fullName, f, ioStreams),
			},
		},
		{
			Message: "Advanced Commands:",
			Commands: []*cobra.Command{
				admin.NewCommandAdmin("adm", fullName+" "+"adm", f, ioStreams),
				kubectlwrappers.NewCmdCreate(fullName, f, ioStreams),
				kubectlwrappers.NewCmdReplace(fullName, f, ioStreams),
				kubectlwrappers.NewCmdApply(fullName, f, ioStreams),
				kubectlwrappers.NewCmdPatch(fullName, f, ioStreams),
				process.NewCmdProcess(fullName, f, ioStreams),
				export.NewCmdExport(fullName, f, ioStreams),
				extract.NewCmdExtract(fullName, f, ioStreams),
				idle.NewCmdIdle(fullName, f, ioStreams),
				observe.NewCmdObserve(fullName, f, ioStreams),
				policy.NewCmdPolicy(policy.PolicyRecommendedName, fullName+" "+policy.PolicyRecommendedName, f, ioStreams),
				kubectlwrappers.NewCmdAuth(fullName, f, ioStreams),
				kubectlwrappers.NewCmdConvert(fullName, f, ioStreams),
				importer.NewCmdImport(fullName, f, ioStreams),
				image.NewCmdImage(fullName, f, ioStreams),
				registry.NewCmd(fullName, f, ioStreams),
				kubectlwrappers.NewCmdApiVersions(fullName, f, ioStreams),
				kubectlwrappers.NewCmdApiResources(fullName, f, ioStreams),
			},
		},
		{
			Message: "Settings Commands:",
			Commands: []*cobra.Command{
				logout.NewCmdLogout("logout", fullName+" logout", fullName+" login", f, ioStreams),
				kubectlwrappers.NewCmdConfig(fullName, "config", f, ioStreams),
				whoami.NewCmdWhoAmI(whoami.WhoAmIRecommendedCommandName, fullName+" "+whoami.WhoAmIRecommendedCommandName, f, ioStreams),
				kubectlwrappers.NewCmdCompletion(fullName, ioStreams),
			},
		},
	}
	groups.Add(cmds)

	ocEditFullName := fullName + " edit"
	ocSecretsFullName := fullName + " " + secrets.SecretsRecommendedName
	ocSecretsNewFullName := ocSecretsFullName + " " + secrets.NewSecretRecommendedCommandName

	filters := []string{
		"options",
		"deploy",
		// These commands are deprecated and should not appear in help
		moved(fullName, "logs", cmds, buildlogs.NewCmdBuildLogs(fullName, f, ioStreams)),
		moved(fullName, "secrets link", secretcmds, secrets.NewCmdLinkSecret("add", fullName, f, ioStreams)),
		moved(fullName, "create secret", secretcmds, secrets.NewCmdCreateSecret(secrets.NewSecretRecommendedCommandName, fullName, f, ioStreams)),
		moved(fullName, "create secret", secretcmds, secrets.NewCmdCreateDockerConfigSecret(secrets.CreateDockerConfigSecretRecommendedName, fullName, f, ioStreams, ocSecretsNewFullName, ocEditFullName)),
		moved(fullName, "create secret", secretcmds, secrets.NewCmdCreateBasicAuthSecret(secrets.CreateBasicAuthSecretRecommendedCommandName, fullName, f, ioStreams, ocSecretsNewFullName, ocEditFullName)),
		moved(fullName, "create secret", secretcmds, secrets.NewCmdCreateSSHAuthSecret(secrets.CreateSSHAuthSecretRecommendedCommandName, fullName, f, ioStreams, ocSecretsNewFullName, ocEditFullName)),
	}

	changeSharedFlagDefaults(cmds)
	templates.ActsAsRootCommand(cmds, filters, groups...).
		ExposeFlags(loginCmd, "certificate-authority", "insecure-skip-tls-verify", "token")

	cmds.AddCommand(newExperimentalCommand("ex", name+" ex", f, ioStreams))

	cmds.AddCommand(kubectlwrappers.NewCmdPlugin(fullName, f, ioStreams))
	if name == fullName {
		cmds.AddCommand(version.NewCmdVersion(fullName, f, version.NewVersionOptions(true, ioStreams)))
	}
	cmds.AddCommand(options.NewCmdOptions(ioStreams))

	if cmds.Flag("namespace") != nil {
		if cmds.Flag("namespace").Annotations == nil {
			cmds.Flag("namespace").Annotations = map[string][]string{}
		}
		cmds.Flag("namespace").Annotations[cobra.BashCompCustom] = append(
			cmds.Flag("namespace").Annotations[cobra.BashCompCustom],
			"__oc_get_namespaces",
		)
	}

	return cmds
}

func moved(fullName, to string, parent, cmd *cobra.Command) string {
	cmd.Long = fmt.Sprintf("DEPRECATED: This command has been moved to \"%s %s\"", fullName, to)
	cmd.Short = fmt.Sprintf("DEPRECATED: %s", to)
	parent.AddCommand(cmd)
	return cmd.Name()
}

// changeSharedFlagDefaults changes values of shared flags that we disagree with.  This can't be done in godep code because
// that would change behavior in our `kubectl` symlink. Defend each change.
// 1. show-all - the most interesting pods are terminated/failed pods.  We don't want to exclude them from printing
func changeSharedFlagDefaults(rootCmd *cobra.Command) {
	cmds := []*cobra.Command{rootCmd}

	for i := 0; i < len(cmds); i++ {
		currCmd := cmds[i]
		cmds = append(cmds, currCmd.Commands()...)

		if showAllFlag := currCmd.Flags().Lookup("show-all"); showAllFlag != nil {
			showAllFlag.DefValue = "true"
			showAllFlag.Value.Set("true")
			showAllFlag.Changed = false
			showAllFlag.Usage = "When printing, show all resources (false means hide terminated pods.)"
		}

		// we want to disable the --validate flag by default when we're running kube commands from oc.  We want to make sure
		// that we're only getting the upstream --validate flags, so check both the flag and the usage
		if validateFlag := currCmd.Flags().Lookup("validate"); (validateFlag != nil) && (validateFlag.Usage == "If true, use a schema to validate the input before sending it") {
			validateFlag.DefValue = "false"
			validateFlag.Value.Set("false")
			validateFlag.Changed = false
		}
	}
}

func newExperimentalCommand(name, fullName string, f kcmdutil.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
	experimental := &cobra.Command{
		Use:   name,
		Short: "Experimental commands under active development",
		Long:  "The commands grouped here are under development and may change without notice.",
		Run: func(c *cobra.Command, args []string) {
			c.SetOutput(ioStreams.Out)
			c.Help()
		},
		BashCompletionFunction: admin.BashCompletionFunc,
	}

	experimental.AddCommand(exipfailover.NewCmdIPFailoverConfig(f, fullName, "ipfailover", ioStreams))
	experimental.AddCommand(dockergc.NewCmdDockerGCConfig(f, fullName, "dockergc", ioStreams))
	experimental.AddCommand(buildchain.NewCmdBuildChain(name, fullName+" "+buildchain.BuildChainRecommendedCommandName, f, ioStreams))
	experimental.AddCommand(configcmd.NewCmdConfig(configcmd.ConfigRecommendedName, fullName+" "+configcmd.ConfigRecommendedName, f, ioStreams))
	deprecatedDiag := diagnostics.NewCmdDiagnostics(diagnostics.DiagnosticsRecommendedName, fullName+" "+diagnostics.DiagnosticsRecommendedName, f, ioStreams)
	deprecatedDiag.Deprecated = fmt.Sprintf(`use "oc adm %[1]s" to run diagnostics instead.`, diagnostics.DiagnosticsRecommendedName)
	experimental.AddCommand(deprecatedDiag)
	experimental.AddCommand(options.NewCmdOptions(ioStreams))

	// these groups also live under `oc adm groups {sync,prune}` and are here only for backwards compatibility
	experimental.AddCommand(sync.NewCmdSync("sync-groups", fullName+" "+"sync-groups", f, ioStreams))
	experimental.AddCommand(sync.NewCmdPrune("prune-groups", fullName+" "+"prune-groups", f, ioStreams))
	return experimental
}

// CommandFor returns the appropriate command for this base name,
// or the OpenShift CLI command.
func CommandFor(basename string) *cobra.Command {
	var cmd *cobra.Command

	in, out, errout := os.Stdin, os.Stdout, os.Stderr

	// Make case-insensitive and strip executable suffix if present
	if runtime.GOOS == "windows" {
		basename = strings.ToLower(basename)
		basename = strings.TrimSuffix(basename, ".exe")
	}

	switch basename {
	case "kubectl":
		kcmdutil.DefaultPrintingScheme = ocscheme.PrintingInternalScheme
		cmd = kubecmd.NewKubectlCommand(in, out, errout)
	case "openshift-deploy":
		cmd = deployer.NewCommandDeployer(basename)
	case "openshift-sti-build":
		cmd = builder.NewCommandS2IBuilder(basename)
	case "openshift-docker-build":
		cmd = builder.NewCommandDockerBuilder(basename)
	case "openshift-git-clone":
		cmd = builder.NewCommandGitClone(basename)
	case "openshift-manage-dockerfile":
		cmd = builder.NewCommandManageDockerfile(basename)
	case "openshift-extract-image-content":
		cmd = builder.NewCommandExtractImageContent(basename)
	case "openshift-router":
		cmd = irouter.NewCommandTemplateRouter(basename)
	case "openshift-f5-router":
		cmd = irouter.NewCommandF5Router(basename)
	case "openshift-recycle":
		cmd = recycle.NewCommandRecycle(basename, out)
	default:
		kcmdutil.DefaultPrintingScheme = ocscheme.PrintingInternalScheme
		shimKubectlForOc()
		cmd = NewCommandCLI("oc", "oc", in, out, errout)
	}

	if cmd.UsageFunc() == nil {
		templates.ActsAsRootCommand(cmd, []string{"options"})
	}
	flagtypes.GLog(cmd.PersistentFlags())

	return cmd
}
