package cli

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/openshift/origin/pkg/api/legacygroupification"
	"github.com/spf13/cobra"

	kubecmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	ktemplates "k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/term"
	"github.com/openshift/origin/pkg/oc/admin"
	diagnostics "github.com/openshift/origin/pkg/oc/admin/diagnostics"
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
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
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

	f := clientcmd.New(cmds.PersistentFlags())

	loginCmd := login.NewCmdLogin(fullName, f, in, out, errout)
	secretcmds := secrets.NewCmdSecrets(secrets.SecretsRecommendedName, fullName+" "+secrets.SecretsRecommendedName, f, out, errout)

	groups := ktemplates.CommandGroups{
		{
			Message: "Basic Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdTypes(fullName, f, out),
				loginCmd,
				cmd.NewCmdRequestProject(cmd.RequestProjectRecommendedCommandName, fullName, f, out, errout),
				cmd.NewCmdNewApplication(cmd.NewAppRecommendedCommandName, fullName, f, in, out, errout),
				cmd.NewCmdStatus(cmd.StatusRecommendedName, fullName, fullName+" "+cmd.StatusRecommendedName, f, out),
				cmd.NewCmdProject(fullName+" project", f, out),
				cmd.NewCmdProjects(fullName, f, out),
				cmd.NewCmdExplain(fullName, f, out, errout),
				cluster.NewCmdCluster(cluster.ClusterRecommendedName, fullName+" "+cluster.ClusterRecommendedName, f, out, errout),
			},
		},
		{
			Message: "Build and Deploy Commands:",
			Commands: []*cobra.Command{
				rollout.NewCmdRollout(fullName, f, out, errout),
				cmd.NewCmdRollback(fullName, f, out),
				cmd.NewCmdNewBuild(cmd.NewBuildRecommendedCommandName, fullName, f, in, out, errout),
				cmd.NewCmdStartBuild(fullName, f, in, out, errout),
				cmd.NewCmdCancelBuild(cmd.CancelBuildRecommendedCommandName, fullName, f, in, out, errout),
				cmd.NewCmdImportImage(fullName, f, out, errout),
				cmd.NewCmdTag(fullName, f, out),
			},
		},
		{
			Message: "Application Management Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdGet(fullName, f, out, errout),
				cmd.NewCmdDescribe(fullName, f, out, errout),
				cmd.NewCmdEdit(fullName, f, out, errout),
				set.NewCmdSet(fullName, f, in, out, errout),
				cmd.NewCmdLabel(fullName, f, out),
				cmd.NewCmdAnnotate(fullName, f, out),
				cmd.NewCmdExpose(fullName, f, out),
				cmd.NewCmdDelete(fullName, f, out, errout),
				cmd.NewCmdScale(fullName, f, out, errout),
				cmd.NewCmdAutoscale(fullName, f, out),
				secretcmds,
				sa.NewCmdServiceAccounts(sa.ServiceAccountsRecommendedName, fullName+" "+sa.ServiceAccountsRecommendedName, f, out, errout),
			},
		},
		{
			Message: "Troubleshooting and Debugging Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdLogs(cmd.LogsRecommendedCommandName, fullName, f, out, errout),
				cmd.NewCmdRsh(cmd.RshRecommendedName, fullName, f, in, out, errout),
				rsync.NewCmdRsync(rsync.RsyncRecommendedName, fullName, f, out, errout),
				cmd.NewCmdPortForward(fullName, f, out, errout),
				cmd.NewCmdDebug(fullName, f, in, out, errout),
				cmd.NewCmdExec(fullName, f, in, out, errout),
				cmd.NewCmdProxy(fullName, f, out),
				cmd.NewCmdAttach(fullName, f, in, out, errout),
				cmd.NewCmdRun(fullName, f, in, out, errout),
				cmd.NewCmdCp(fullName, f, in, out, errout),
			},
		},
		{
			Message: "Advanced Commands:",
			Commands: []*cobra.Command{
				admin.NewCommandAdmin("adm", fullName+" "+"adm", in, out, errout),
				cmd.NewCmdCreate(fullName, f, out, errout),
				cmd.NewCmdReplace(fullName, f, out),
				cmd.NewCmdApply(fullName, f, out, errout),
				cmd.NewCmdPatch(fullName, f, out),
				cmd.NewCmdProcess(fullName, f, in, out, errout),
				cmd.NewCmdExport(fullName, f, in, out),
				cmd.NewCmdExtract(fullName, f, in, out, errout),
				cmd.NewCmdIdle(fullName, f, out, errout),
				observe.NewCmdObserve(fullName, f, out, errout),
				policy.NewCmdPolicy(policy.PolicyRecommendedName, fullName+" "+policy.PolicyRecommendedName, f, out, errout),
				cmd.NewCmdAuth(fullName, f, out, errout),
				cmd.NewCmdConvert(fullName, f, out),
				importer.NewCmdImport(fullName, f, in, out, errout),
				image.NewCmdImage(fullName, f, in, out, errout),
				registry.NewCmd(fullName, f, in, out, errout),
			},
		},
		{
			Message: "Settings Commands:",
			Commands: []*cobra.Command{
				login.NewCmdLogout("logout", fullName+" logout", fullName+" login", f, in, out),
				cmd.NewCmdConfig(fullName, "config", f, out, errout),
				cmd.NewCmdWhoAmI(cmd.WhoAmIRecommendedCommandName, fullName+" "+cmd.WhoAmIRecommendedCommandName, f, out),
				cmd.NewCmdCompletion(fullName, out),
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
		moved(fullName, "set env", cmds, set.NewCmdEnv(fullName, f, in, out, errout)),
		moved(fullName, "set volume", cmds, set.NewCmdVolume(fullName, f, out, errout)),
		moved(fullName, "logs", cmds, cmd.NewCmdBuildLogs(fullName, f, out)),
		moved(fullName, "secrets link", secretcmds, secrets.NewCmdLinkSecret("add", fullName, f, out)),
		moved(fullName, "create secret", secretcmds, secrets.NewCmdCreateSecret(secrets.NewSecretRecommendedCommandName, fullName, f, out)),
		moved(fullName, "create secret", secretcmds, secrets.NewCmdCreateDockerConfigSecret(secrets.CreateDockerConfigSecretRecommendedName, fullName, f, out, ocSecretsNewFullName, ocEditFullName)),
		moved(fullName, "create secret", secretcmds, secrets.NewCmdCreateBasicAuthSecret(secrets.CreateBasicAuthSecretRecommendedCommandName, fullName, f, in, out, ocSecretsNewFullName, ocEditFullName)),
		moved(fullName, "create secret", secretcmds, secrets.NewCmdCreateSSHAuthSecret(secrets.CreateSSHAuthSecretRecommendedCommandName, fullName, f, out, ocSecretsNewFullName, ocEditFullName)),
	}

	changeSharedFlagDefaults(cmds)
	templates.ActsAsRootCommand(cmds, filters, groups...).
		ExposeFlags(loginCmd, "certificate-authority", "insecure-skip-tls-verify", "token")

	cmds.AddCommand(newExperimentalCommand("ex", name+"ex"))

	cmds.AddCommand(cmd.NewCmdPlugin(fullName, f, in, out, errout))
	if name == fullName {
		cmds.AddCommand(cmd.NewCmdVersion(fullName, f, out, cmd.VersionOptions{PrintClientFeatures: true}))
	}
	cmds.AddCommand(cmd.NewCmdOptions(out))
	cmds.AddCommand(cmd.NewCmdDeploy(fullName, f, out))

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

func newExperimentalCommand(name, fullName string) *cobra.Command {
	out := os.Stdout
	errout := os.Stderr

	experimental := &cobra.Command{
		Use:   name,
		Short: "Experimental commands under active development",
		Long:  "The commands grouped here are under development and may change without notice.",
		Run: func(c *cobra.Command, args []string) {
			c.SetOutput(out)
			c.Help()
		},
		BashCompletionFunction: admin.BashCompletionFunc,
	}

	f := clientcmd.New(experimental.PersistentFlags())

	experimental.AddCommand(exipfailover.NewCmdIPFailoverConfig(f, fullName, "ipfailover", out, errout))
	experimental.AddCommand(dockergc.NewCmdDockerGCConfig(f, fullName, "dockergc", out, errout))
	experimental.AddCommand(buildchain.NewCmdBuildChain(name, fullName+" "+buildchain.BuildChainRecommendedCommandName, f, out))
	experimental.AddCommand(configcmd.NewCmdConfig(configcmd.ConfigRecommendedName, fullName+" "+configcmd.ConfigRecommendedName, f, out, errout))
	deprecatedDiag := diagnostics.NewCmdDiagnostics(diagnostics.DiagnosticsRecommendedName, fullName+" "+diagnostics.DiagnosticsRecommendedName, out)
	deprecatedDiag.Deprecated = fmt.Sprintf(`use "oc adm %[1]s" to run diagnostics instead.`, diagnostics.DiagnosticsRecommendedName)
	experimental.AddCommand(deprecatedDiag)
	experimental.AddCommand(cmd.NewCmdOptions(out))

	// these groups also live under `oc adm groups {sync,prune}` and are here only for backwards compatibility
	experimental.AddCommand(sync.NewCmdSync("sync-groups", fullName+" "+"sync-groups", f, out))
	experimental.AddCommand(sync.NewCmdPrune("prune-groups", fullName+" "+"prune-groups", f, out))
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
		cmd = kubecmd.NewKubectlCommand(kcmdutil.NewFactory(nil), in, out, errout)
	default:
		// we only need this change for `oc`.  `kubectl` should behave as close to `kubectl` as we can
		resource.OAPIToGroupified = legacygroupification.OAPIToGroupified
		kcmdutil.OAPIToGroupifiedGVK = legacygroupification.OAPIToGroupifiedGVK
		cmd = NewCommandCLI("oc", "oc", in, out, errout)
	}

	if cmd.UsageFunc() == nil {
		templates.ActsAsRootCommand(cmd, []string{"options"})
	}
	flagtypes.GLog(cmd.PersistentFlags())

	return cmd
}
