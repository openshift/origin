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

	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/cli/policy"
	"github.com/openshift/origin/pkg/cmd/cli/secrets"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/version"
)

const cliLong = `
Developer and Administrator Client

This client exposes commands for managing your applications, as well as lower level
tools to interact with each component of your system.

To create a new application, you can use the example app source. Login to your server and then
run new-app:

  $ %[1]s login
  $ %[1]s new-app openshift/ruby-20-centos7~https://github.com/openshift/ruby-hello-world.git

This will create an application based on the Docker image 'openshift/ruby-20-centos7' that builds
the source code at 'github.com/openshift/ruby-hello-world.git'. A build will start automatically and
a deployment will start as soon as the build finishes.

Once your application is deployed, use the status, get, and describe commands to see more about
the created components:

  $ %[1]s status
  $ %[1]s describe deploymentconfig ruby-hello-world
  $ %[1]s get pods

You'll be able to view the deployed application on the IP and port of the service that new-app
created for you.

You can easily switch between multiple projects using '%[1]s project <projectname>'.`

func NewCommandCLI(name, fullName string, in io.Reader, out, errout io.Writer) *cobra.Command {
	// Main command
	cmds := &cobra.Command{
		Use:   name,
		Short: "Command line tools for managing applications",
		Long:  fmt.Sprintf(cliLong, fullName),
		Run:   cmdutil.DefaultSubCommandRun(out),
		BashCompletionFunction: bashCompletionFunc,
	}

	f := clientcmd.New(cmds.PersistentFlags())

	loginCmd := cmd.NewCmdLogin(fullName, f, in, out)
	groups := templates.CommandGroups{
		{
			Message: "Basic Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdTypes(fullName, f, out),
				loginCmd,
				cmd.NewCmdRequestProject("new-project", fullName+" new-project", fullName+" login", fullName+" project", f, out),
				cmd.NewCmdNewApplication(fullName, f, out),
				cmd.NewCmdStatus(cmd.StatusRecommendedName, fullName+" "+cmd.StatusRecommendedName, f, out),
				cmd.NewCmdProject(fullName+" project", f, out),
			},
		},
		{
			Message: "Build and Deploy Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdStartBuild(fullName, f, in, out),
				cmd.NewCmdBuildLogs(fullName, f, out),
				cmd.NewCmdDeploy(fullName, f, out),
				cmd.NewCmdRollback(fullName, f, out),
				cmd.NewCmdNewBuild(fullName, f, in, out),
				cmd.NewCmdCancelBuild(fullName, f, out),
				cmd.NewCmdImportImage(fullName, f, out),
				cmd.NewCmdScale(fullName, f, out),
				cmd.NewCmdTag(fullName, f, out),
			},
		},
		{
			Message: "Application Modification Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdGet(fullName, f, out),
				cmd.NewCmdDescribe(fullName, f, out),
				cmd.NewCmdEdit(fullName, f, out),
				cmd.NewCmdEnv(fullName, f, in, out),
				cmd.NewCmdVolume(fullName, f, out, errout),
				cmd.NewCmdLabel(fullName, f, out),
				cmd.NewCmdAnnotate(fullName, f, out),
				cmd.NewCmdExpose(fullName, f, out),
				cmd.NewCmdStop(fullName, f, out),
				cmd.NewCmdDelete(fullName, f, out),
			},
		},
		{
			Message: "Troubleshooting and Debugging Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdExplain(fullName, f, out),
				cmd.NewCmdLogs(fullName, f, out),
				cmd.NewCmdRsh(cmd.RshRecommendedName, fullName, f, in, out, errout),
				cmd.NewCmdRsync(cmd.RsyncRecommendedName, fullName, f, out, errout),
				cmd.NewCmdExec(fullName, f, in, out, errout),
				cmd.NewCmdPortForward(fullName, f),
				cmd.NewCmdProxy(fullName, f, out),
			},
		},
		{
			Message: "Advanced Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdCreate(fullName, f, out),
				cmd.NewCmdReplace(fullName, f, out),
				// TODO decide what to do about apply.  Its doing unusual things
				// cmd.NewCmdApply(fullName, f, out),
				cmd.NewCmdPatch(fullName, f, out),
				cmd.NewCmdProcess(fullName, f, out),
				cmd.NewCmdExport(fullName, f, in, out),
				cmd.NewCmdRun(fullName, f, in, out, errout),
				cmd.NewCmdAttach(fullName, f, in, out, errout),
				policy.NewCmdPolicy(policy.PolicyRecommendedName, fullName+" "+policy.PolicyRecommendedName, f, out),
				secrets.NewCmdSecrets(secrets.SecretsRecommendedName, fullName+" "+secrets.SecretsRecommendedName, f, in, out, fullName+" edit"),
				cmd.NewCmdConvert(fullName, f, out),
			},
		},
		{
			Message: "Settings Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdLogout("logout", fullName+" logout", fullName+" login", f, in, out),
				cmd.NewCmdConfig(fullName, "config"),
				cmd.NewCmdWhoAmI(cmd.WhoAmIRecommendedCommandName, fullName+" "+cmd.WhoAmIRecommendedCommandName, f, out),
			},
		},
	}
	groups.Add(cmds)
	changeSharedFlagDefaults(cmds)
	templates.ActsAsRootCommand(cmds, groups...).
		ExposeFlags(loginCmd, "certificate-authority", "insecure-skip-tls-verify")

	if name == fullName {
		cmds.AddCommand(version.NewVersionCommand(fullName, false))
	}
	cmds.AddCommand(cmd.NewCmdOptions(out))

	return cmds
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
	templates.ActsAsRootCommand(cmds)
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
		templates.ActsAsRootCommand(cmd)
	}
	flagtypes.GLog(cmd.PersistentFlags())

	return cmd
}
