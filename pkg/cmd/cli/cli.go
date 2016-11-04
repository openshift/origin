package cli

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	kubecmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/admin"
	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/cli/cmd/cluster"
	"github.com/openshift/origin/pkg/cmd/cli/cmd/dockerbuild"
	"github.com/openshift/origin/pkg/cmd/cli/cmd/importer"
	"github.com/openshift/origin/pkg/cmd/cli/cmd/login"
	"github.com/openshift/origin/pkg/cmd/cli/cmd/observe"
	"github.com/openshift/origin/pkg/cmd/cli/cmd/rollout"
	"github.com/openshift/origin/pkg/cmd/cli/cmd/rsync"
	"github.com/openshift/origin/pkg/cmd/cli/cmd/set"
	"github.com/openshift/origin/pkg/cmd/cli/policy"
	"github.com/openshift/origin/pkg/cmd/cli/sa"
	"github.com/openshift/origin/pkg/cmd/cli/secrets"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/term"
)

const productName = `OpenShift`

var (
	cliLong = templates.LongDesc(`
    ` + productName + ` Client

    This client helps you develop, build, deploy, and run your applications on any
    OpenShift or Kubernetes compatible platform. It also includes the administrative
    commands for managing a cluster under the 'adm' subcommand.`)

	cliExplain = templates.LongDesc(`
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

	loginCmd := login.NewCmdLogin(fullName, f, in, out)
	secretcmds := secrets.NewCmdSecrets(secrets.SecretsRecommendedName, fullName+" "+secrets.SecretsRecommendedName, f, in, out, errout, fullName+" edit")

	groups := templates.CommandGroups{
		{
			Message: "Basic Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdTypes(fullName, f, out),
				loginCmd,
				cmd.NewCmdRequestProject(cmd.RequestProjectRecommendedCommandName, fullName, f, out, errout),
				cmd.NewCmdNewApplication(cmd.NewAppRecommendedCommandName, fullName, f, out, errout),
				cmd.NewCmdStatus(cmd.StatusRecommendedName, fullName, fullName+" "+cmd.StatusRecommendedName, f, out),
				cmd.NewCmdProject(fullName+" project", f, out),
				cmd.NewCmdProjects(fullName, f, out),
				cmd.NewCmdExplain(fullName, f, out, errout),
				cluster.NewCmdCluster(cluster.ClusterRecommendedName, fullName+" "+cluster.ClusterRecommendedName, f, out, errout),
				cmd.NewCmdIdle(fullName, f, out, errout),
			},
		},
		{
			Message: "Build and Deploy Commands:",
			Commands: []*cobra.Command{
				rollout.NewCmdRollout(fullName, f, out, errout),
				cmd.NewCmdDeploy(fullName, f, out),
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
				cmd.NewCmdDelete(fullName, f, out),
				cmd.NewCmdScale(fullName, f, out),
				cmd.NewCmdAutoscale(fullName, f, out),
				secretcmds,
				sa.NewCmdServiceAccounts(sa.ServiceAccountsRecommendedName, fullName+" "+sa.ServiceAccountsRecommendedName, f, out, errout),
			},
		},
		{
			Message: "Troubleshooting and Debugging Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdLogs(cmd.LogsRecommendedCommandName, fullName, f, out),
				cmd.NewCmdRsh(cmd.RshRecommendedName, fullName, f, in, out, errout),
				rsync.NewCmdRsync(rsync.RsyncRecommendedName, fullName, f, out, errout),
				cmd.NewCmdPortForward(fullName, f, out, errout),
				cmd.NewCmdDebug(fullName, f, in, out, errout),
				cmd.NewCmdExec(fullName, f, in, out, errout),
				cmd.NewCmdProxy(fullName, f, out),
				cmd.NewCmdAttach(fullName, f, in, out, errout),
				cmd.NewCmdRun(fullName, f, in, out, errout),
			},
		},
		{
			Message: "Advanced Commands:",
			Commands: []*cobra.Command{
				admin.NewCommandAdmin("adm", fullName+" "+"adm", in, out, errout),
				cmd.NewCmdCreate(fullName, f, out, errout),
				cmd.NewCmdReplace(fullName, f, out),
				cmd.NewCmdApply(fullName, f, out),
				cmd.NewCmdPatch(fullName, f, out),
				cmd.NewCmdProcess(fullName, f, out, errout),
				cmd.NewCmdExport(fullName, f, in, out),
				cmd.NewCmdExtract(fullName, f, in, out, errout),
				observe.NewCmdObserve(fullName, f, out, errout),
				policy.NewCmdPolicy(policy.PolicyRecommendedName, fullName+" "+policy.PolicyRecommendedName, f, out, errout),
				cmd.NewCmdConvert(fullName, f, out),
				importer.NewCmdImport(fullName, f, in, out, errout),
			},
		},
		{
			Message: "Settings Commands:",
			Commands: []*cobra.Command{
				login.NewCmdLogout("logout", fullName+" logout", fullName+" login", f, in, out),
				cmd.NewCmdConfig(fullName, "config", out, errout),
				cmd.NewCmdWhoAmI(cmd.WhoAmIRecommendedCommandName, fullName+" "+cmd.WhoAmIRecommendedCommandName, f, out),
				cmd.NewCmdCompletion(fullName, f, out),
			},
		},
	}
	groups.Add(cmds)

	filters := []string{
		"options",
		// These commands are deprecated and should not appear in help
		moved(fullName, "set env", cmds, set.NewCmdEnv(fullName, f, in, out, errout)),
		moved(fullName, "set volume", cmds, set.NewCmdVolume(fullName, f, out, errout)),
		moved(fullName, "logs", cmds, cmd.NewCmdBuildLogs(fullName, f, out)),
		moved(fullName, "secrets link", secretcmds, secrets.NewCmdLinkSecret("add", fullName, f.Factory, out)),
	}

	changeSharedFlagDefaults(cmds)
	templates.ActsAsRootCommand(cmds, filters, groups...).
		ExposeFlags(loginCmd, "certificate-authority", "insecure-skip-tls-verify", "token")

	// experimental commands are those that are bundled with the binary but not displayed to end users
	// directly
	experimental := &cobra.Command{
		Use: "ex", // Because this command exposes no description, it will not be shown in help
	}
	experimental.AddCommand(
		dockerbuild.NewCmdDockerbuild(fullName, f, out, errout),
	)
	cmds.AddCommand(experimental)

	if name == fullName {
		cmds.AddCommand(cmd.NewCmdVersion(fullName, f, out, cmd.VersionOptions{PrintClientFeatures: true}))
	}
	cmds.AddCommand(cmd.NewCmdOptions(out))

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

// NewCmdKubectl provides exactly the functionality from Kubernetes,
// but with support for OpenShift resources
func NewCmdKubectl(name string, out io.Writer) *cobra.Command {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	f := clientcmd.New(flags)
	cmds := kubecmd.NewKubectlCommand(f.Factory, os.Stdin, out, os.Stderr)
	cmds.Aliases = []string{"kubectl"}
	cmds.Use = name
	cmds.Short = "Kubernetes cluster management via kubectl"
	flags.VisitAll(func(flag *pflag.Flag) {
		if f := cmds.PersistentFlags().Lookup(flag.Name); f == nil {
			cmds.PersistentFlags().AddFlag(flag)
		} else {
			glog.V(5).Infof("already registered flag %s", flag.Name)
		}
	})
	cmds.PersistentFlags().Var(flags.Lookup("config").Value, "kubeconfig", "Specify a kubeconfig file to define the configuration")
	templates.ActsAsRootCommand(cmds, []string{"options"})
	cmds.AddCommand(cmd.NewCmdOptions(out))
	return cmds
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
		cmd = NewCmdKubectl(basename, out)
	default:
		cmd = NewCommandCLI(basename, basename, in, out, errout)
	}

	if cmd.UsageFunc() == nil {
		templates.ActsAsRootCommand(cmd, []string{"options"})
	}
	flagtypes.GLog(cmd.PersistentFlags())

	return cmd
}
