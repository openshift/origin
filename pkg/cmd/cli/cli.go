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

	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"

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
OpenShift Client

The OpenShift client exposes commands for managing your applications, as well as lower level
tools to interact with each component of your system.

To create a new application, you can use the example app source. Login to your server and then
run new-app:

  $ %[1]s login
  $ %[1]s new-app openshift/ruby-20-centos7~https://github.com/openshift/ruby-hello-world.git

This will create an application based on the Docker image 'openshift/ruby-20-centos7' that builds
the source code at 'github.com/openshift/ruby-hello-world.git'. To start the build, run

  $ %[1]s start-build ruby-hello-world --follow

Once your application is deployed, use the status, get, and describe commands to see more about
the created components:

  $ %[1]s status
  $ %[1]s describe deploymentconfig ruby-hello-world
  $ %[1]s get pods

You'll be able to view the deployed application on the IP and port of the service that new-app
created for you.

You can easily switch between multiple projects using '%[1]s project <projectname>'.`

func NewCommandCLI(name, fullName string) *cobra.Command {
	in := os.Stdin
	out := os.Stdout
	errout := os.Stderr

	// Main command
	cmds := &cobra.Command{
		Use:   name,
		Short: "Client tools for OpenShift",
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
				cmd.NewCmdStatus(fullName, f, out),
				cmd.NewCmdProject(fullName+" project", f, out),
			},
		},
		{
			Message: "Build and Deploy Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdStartBuild(fullName, f, out),
				cmd.NewCmdBuildLogs(fullName, f, out),
				cmd.NewCmdDeploy(fullName, f, out),
				cmd.NewCmdRollback(fullName, f, out),
				cmd.NewCmdNewBuild(fullName, f, out),
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
				cmd.NewCmdVolume(fullName, f, out),
				cmd.NewCmdLabel(fullName, f, out),
				cmd.NewCmdExpose(fullName, f, out),
				cmd.NewCmdStop(fullName, f, out),
				cmd.NewCmdDelete(fullName, f, out),
			},
		},
		{
			Message: "Troubleshooting and Debugging Commands:",
			Commands: []*cobra.Command{
				cmd.NewCmdLogs(fullName, f, out),
				cmd.NewCmdRsh(fullName, f, in, out, errout),
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
				cmd.NewCmdPatch(fullName, f, out),
				cmd.NewCmdProcess(fullName, f, out),
				cmd.NewCmdExport(fullName, f, in, out),
				policy.NewCmdPolicy(policy.PolicyRecommendedName, fullName+" "+policy.PolicyRecommendedName, f, out),
				secrets.NewCmdSecrets(secrets.SecretsRecommendedName, fullName+" "+secrets.SecretsRecommendedName, f, out, fullName+" edit"),
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
	templates.ActsAsRootCommand(cmds, groups...).
		ExposeFlags(loginCmd, "certificate-authority", "insecure-skip-tls-verify")

	if name == fullName {
		cmds.AddCommand(version.NewVersionCommand(fullName))
	}
	cmds.AddCommand(cmd.NewCmdOptions(out))

	return cmds
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
	cmds.Long = cmds.Long + "\n\nThis command is provided for direct management of the Kubernetes cluster OpenShift runs on."
	flags.VisitAll(func(flag *pflag.Flag) {
		if f := cmds.PersistentFlags().Lookup(flag.Name); f == nil {
			cmds.PersistentFlags().AddFlag(flag)
		} else {
			glog.V(5).Infof("already registered flag %s", flag.Name)
		}
	})
	templates.ActsAsRootCommand(cmds)
	cmds.AddCommand(cmd.NewCmdOptions(out))
	return cmds
}

// CommandFor returns the appropriate command for this base name,
// or the OpenShift CLI command.
func CommandFor(basename string) *cobra.Command {
	var cmd *cobra.Command

	out := os.Stdout

	// Make case-insensitive and strip executable suffix if present
	if runtime.GOOS == "windows" {
		basename = strings.ToLower(basename)
		basename = strings.TrimSuffix(basename, ".exe")
	}

	switch basename {
	case "kubectl":
		cmd = NewCmdKubectl(basename, out)
	default:
		cmd = NewCommandCLI(basename, basename)
	}

	if cmd.UsageFunc() == nil {
		templates.ActsAsRootCommand(cmd)
	}
	flagtypes.GLog(cmd.PersistentFlags())

	return cmd
}

// applyToCreate injects the deprecation notice about for 'apply' command into
// 'create' command.
// TODO: Remove this once we get rid of 'apply' in all documentation/etc.
func applyToCreate(dst *cobra.Command) *cobra.Command {
	dst.Aliases = append(dst.Aliases, "apply")
	oldRun := dst.Run
	dst.Run = func(c *cobra.Command, args []string) {
		calledApply := false
		calledApply = calledApply || len(os.Args) >= 2 && os.Args[1] == "apply" // `oc apply`
		calledApply = calledApply || len(os.Args) >= 3 && os.Args[2] == "apply" // `openshift cli apply`
		if calledApply {
			glog.Errorf("DEPRECATED: The 'apply' command is now deprecated, use 'create' instead.")
		}
		oldRun(c, args)
	}
	return dst
}
