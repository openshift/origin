package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oc/util/ocscheme"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes/scheme"
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
	"github.com/openshift/origin/pkg/oc/admin"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics"
	sync "github.com/openshift/origin/pkg/oc/admin/groups/sync/cli"
	"github.com/openshift/origin/pkg/oc/cli/cmd"
	"github.com/openshift/origin/pkg/oc/cli/cmd/cluster"
	"github.com/openshift/origin/pkg/oc/cli/cmd/image"
	"github.com/openshift/origin/pkg/oc/cli/cmd/importer"
	"github.com/openshift/origin/pkg/oc/cli/cmd/login"
	"github.com/openshift/origin/pkg/oc/cli/cmd/observe"
	"github.com/openshift/origin/pkg/oc/cli/cmd/registry"
	"github.com/openshift/origin/pkg/oc/cli/cmd/rollout"
	"github.com/openshift/origin/pkg/oc/cli/cmd/rsync"
	"github.com/openshift/origin/pkg/oc/cli/cmd/set"
	"github.com/openshift/origin/pkg/oc/cli/policy"
	"github.com/openshift/origin/pkg/oc/cli/sa"
	"github.com/openshift/origin/pkg/oc/cli/secrets"
	"github.com/openshift/origin/pkg/oc/experimental/buildchain"
	configcmd "github.com/openshift/origin/pkg/oc/experimental/config"
	"github.com/openshift/origin/pkg/oc/experimental/dockergc"
	exipfailover "github.com/openshift/origin/pkg/oc/experimental/ipfailover"
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
        %[1]s new-app centos/ruby-22-centos7~https://github.com/openshift/ruby-ex.git
        %[1]s logs -f bc/ruby-ex

    This will create an application based on the Docker image 'centos/ruby-22-centos7' that builds the source code from GitHub. A build will start automatically, push the resulting image to the registry, and a deployment will roll that change out in your project.

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
				cmd.NewCmdTypes(fullName, f, ioStreams),
				loginCmd,
				cmd.NewCmdRequestProject(cmd.RequestProjectRecommendedCommandName, fullName, f, ioStreams),
				cmd.NewCmdNewApplication(cmd.NewAppRecommendedCommandName, fullName, f, ioStreams),
				cmd.NewCmdStatus(cmd.StatusRecommendedName, fullName, fullName+" "+cmd.StatusRecommendedName, f, ioStreams),
				cmd.NewCmdProject(fullName+" project", f, ioStreams),
				cmd.NewCmdProjects(fullName, f, ioStreams),
				cmd.NewCmdExplain(fullName, f, ioStreams),
				cluster.NewCmdCluster(cluster.ClusterRecommendedName, fullName+" "+cluster.ClusterRecommendedName, f, ioStreams),
			},
		},
		{
			Message: "Build and Deploy Commands:",
			Commands: []*cobra.Command{
				rollout.NewCmdRollout(fullName, f, ioStreams),
				cmd.NewCmdRollback(fullName, f, ioStreams),
				cmd.NewCmdNewBuild(cmd.NewBuildRecommendedCommandName, fullName, f, ioStreams),
				cmd.NewCmdStartBuild(fullName, f, ioStreams),
				cmd.NewCmdCancelBuild(cmd.CancelBuildRecommendedCommandName, fullName, f, ioStreams),
				cmd.NewCmdImportImage(fullName, f, ioStreams),
				cmd.NewCmdTag(fullName, f, ioStreams),
			},
		},
		{
			Message: "Application Management Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdGet(fullName, f, ioStreams),
				cmd.NewCmdDescribe(fullName, f, ioStreams),
				cmd.NewCmdEdit(fullName, f, ioStreams),
				set.NewCmdSet(fullName, f, ioStreams),
				cmd.NewCmdLabel(fullName, f, ioStreams),
				cmd.NewCmdAnnotate(fullName, f, ioStreams),
				cmd.NewCmdExpose(fullName, f, ioStreams),
				cmd.NewCmdDelete(fullName, f, ioStreams),
				cmd.NewCmdScale(fullName, f, ioStreams),
				cmd.NewCmdAutoscale(fullName, f, ioStreams),
				secretcmds,
				sa.NewCmdServiceAccounts(sa.ServiceAccountsRecommendedName, fullName+" "+sa.ServiceAccountsRecommendedName, f, ioStreams),
			},
		},
		{
			Message: "Troubleshooting and Debugging Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdLogs(cmd.LogsRecommendedCommandName, fullName, f, ioStreams),
				cmd.NewCmdRsh(cmd.RshRecommendedName, fullName, f, ioStreams),
				rsync.NewCmdRsync(rsync.RsyncRecommendedName, fullName, f, ioStreams),
				cmd.NewCmdPortForward(fullName, f, ioStreams),
				cmd.NewCmdDebug(fullName, f, ioStreams),
				cmd.NewCmdExec(fullName, f, ioStreams),
				cmd.NewCmdProxy(fullName, f, ioStreams),
				cmd.NewCmdAttach(fullName, f, ioStreams),
				cmd.NewCmdRun(fullName, f, ioStreams),
				cmd.NewCmdCp(fullName, f, ioStreams),
				cmd.NewCmdWait(fullName, f, ioStreams),
			},
		},
		{
			Message: "Advanced Commands:",
			Commands: []*cobra.Command{
				admin.NewCommandAdmin("adm", fullName+" "+"adm", f, ioStreams),
				cmd.NewCmdCreate(fullName, f, ioStreams),
				cmd.NewCmdReplace(fullName, f, ioStreams),
				cmd.NewCmdApply(fullName, f, ioStreams),
				cmd.NewCmdPatch(fullName, f, ioStreams),
				cmd.NewCmdProcess(fullName, f, ioStreams),
				cmd.NewCmdExport(fullName, f, ioStreams),
				cmd.NewCmdExtract(fullName, f, ioStreams),
				cmd.NewCmdIdle(fullName, f, ioStreams),
				observe.NewCmdObserve(fullName, f, ioStreams),
				policy.NewCmdPolicy(policy.PolicyRecommendedName, fullName+" "+policy.PolicyRecommendedName, f, ioStreams),
				cmd.NewCmdAuth(fullName, f, ioStreams),
				cmd.NewCmdConvert(fullName, f, ioStreams),
				importer.NewCmdImport(fullName, f, ioStreams),
				image.NewCmdImage(fullName, f, ioStreams),
				registry.NewCmd(fullName, f, ioStreams),
				cmd.NewCmdApiVersions(fullName, f, ioStreams),
				cmd.NewCmdApiResources(fullName, f, ioStreams),
			},
		},
		{
			Message: "Settings Commands:",
			Commands: []*cobra.Command{
				login.NewCmdLogout("logout", fullName+" logout", fullName+" login", f, ioStreams),
				cmd.NewCmdConfig(fullName, "config", f, ioStreams),
				cmd.NewCmdWhoAmI(cmd.WhoAmIRecommendedCommandName, fullName+" "+cmd.WhoAmIRecommendedCommandName, f, ioStreams),
				cmd.NewCmdCompletion(fullName, ioStreams),
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
		moved(fullName, "logs", cmds, cmd.NewCmdBuildLogs(fullName, f, ioStreams)),
		moved(fullName, "secrets link", secretcmds, secrets.NewCmdLinkSecret("add", fullName, f, ioStreams)),
		moved(fullName, "create secret", secretcmds, secrets.NewCmdCreateSecret(secrets.NewSecretRecommendedCommandName, fullName, f, ioStreams)),
		moved(fullName, "create secret", secretcmds, secrets.NewCmdCreateDockerConfigSecret(secrets.CreateDockerConfigSecretRecommendedName, fullName, f, ioStreams, ocSecretsNewFullName, ocEditFullName)),
		moved(fullName, "create secret", secretcmds, secrets.NewCmdCreateBasicAuthSecret(secrets.CreateBasicAuthSecretRecommendedCommandName, fullName, f, ioStreams, ocSecretsNewFullName, ocEditFullName)),
		moved(fullName, "create secret", secretcmds, secrets.NewCmdCreateSSHAuthSecret(secrets.CreateSSHAuthSecretRecommendedCommandName, fullName, f, ioStreams, ocSecretsNewFullName, ocEditFullName)),
	}

	changeSharedFlagDefaults(cmds)
	templates.ActsAsRootCommand(cmds, filters, groups...).
		ExposeFlags(loginCmd, "certificate-authority", "insecure-skip-tls-verify", "token")

	cmds.AddCommand(newExperimentalCommand("ex", name+"ex", f, ioStreams))

	cmds.AddCommand(cmd.NewCmdPlugin(fullName, f, ioStreams))
	if name == fullName {
		cmds.AddCommand(cmd.NewCmdVersion(fullName, f, ioStreams, cmd.VersionOptions{PrintClientFeatures: true}))
	}
	cmds.AddCommand(cmd.NewCmdOptions(ioStreams))
	cmds.AddCommand(cmd.NewCmdDeploy(fullName, f, ioStreams))

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
	experimental.AddCommand(cmd.NewCmdOptions(ioStreams))

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

	if err := ocscheme.AddOpenShiftExternalToScheme(scheme.Scheme); err != nil {
		glog.Fatal(err)
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
