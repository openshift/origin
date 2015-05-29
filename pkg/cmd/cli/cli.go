package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"

	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/version"
)

const cli_long = `OpenShift Client.

The OpenShift client exposes commands for managing your applications, as well as lower level
tools to interact with each component of your system.

To create a new application, you can use the example app source. Login to your server and then
run new-app:

  $ %[1]s login
  $ %[1]s new-app openshift/ruby-20-centos7~git@github.com/mfojtik/sinatra-app-example

This will create an application based on the Docker image 'openshift/ruby-20-centos7' that builds
the source code at 'github.com/mfojtik/sinatra-app-example'. To start the build, run

  $ %[1]s start-build sinatra-app-example

and watch the build logs and build status with:

  $ %[1]s get builds
  $ %[1]s build-logs <name_of_build>

You'll be able to view the deployed application on the IP and port of the service that new-app
created for you.

You can easily switch between multiple projects using '%[1]s project <projectname>'.

Note: This is a beta release of OpenShift and may change significantly.  See
    https://github.com/openshift/origin for the latest information on OpenShift.`

func NewCommandCLI(name, fullName string) *cobra.Command {
	// Main command
	cmds := &cobra.Command{
		Use:   name,
		Short: "Client tools for OpenShift",
		Long:  fmt.Sprintf(cli_long, fullName),
		Run: func(c *cobra.Command, args []string) {
			c.SetOutput(os.Stdout)
			c.Help()
		},
		BashCompletionFunction: bashCompletionFunc,
	}

	f := clientcmd.New(cmds.PersistentFlags())
	in := os.Stdin
	out := os.Stdout

	cmds.AddCommand(cmd.NewCmdLogin(fullName, f, in, out))
	cmds.AddCommand(cmd.NewCmdLogout("logout", fullName+" logout", fullName+" login", f, in, out))
	cmds.AddCommand(cmd.NewCmdProject(fullName+" project", f, out))
	cmds.AddCommand(cmd.NewCmdRequestProject("new-project", fullName+" new-project", fullName+" login", fullName+" project", f, out))
	cmds.AddCommand(cmd.NewCmdNewApplication(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdStatus(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdStartBuild(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdCancelBuild(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdBuildLogs(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdDeploy(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdRollback(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdEnv(fullName, f, os.Stdin, out))
	cmds.AddCommand(cmd.NewCmdExpose(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdGet(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdDescribe(fullName, f, out))
	// Deprecate 'osc apply' with 'osc create' command.
	cmds.AddCommand(applyToCreate(cmd.NewCmdCreate(fullName, f, out)))
	cmds.AddCommand(cmd.NewCmdProcess(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdEdit(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdUpdate(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdDelete(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdLog(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdExec(fullName, f, os.Stdin, out, os.Stderr))
	cmds.AddCommand(cmd.NewCmdPortForward(fullName, f))
	cmds.AddCommand(cmd.NewCmdProxy(fullName, f, out))
	if name == fullName {
		cmds.AddCommand(version.NewVersionCommand(fullName))
	}
	cmds.AddCommand(cmd.NewCmdConfig(fullName, "config"))
	cmds.AddCommand(cmd.NewCmdOptions(out))
	cmds.AddCommand(cmd.NewCmdScale(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdStop(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdLabel(fullName, f, out))

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
	cmds.AddCommand(cmd.NewCmdOptions(out))
	return cmds
}

// applyToCreate injects the deprecation notice about for 'apply' command into
// 'create' command.
// TODO: Remove this once we get rid of 'apply' in all documentation/etc.
func applyToCreate(dst *cobra.Command) *cobra.Command {
	dst.Aliases = append(dst.Aliases, "apply")
	oldRun := dst.Run
	dst.Run = func(c *cobra.Command, args []string) {
		calledApply := false
		calledApply = calledApply || len(os.Args) >= 2 && os.Args[1] == "apply" // `osc apply`
		calledApply = calledApply || len(os.Args) >= 3 && os.Args[2] == "apply" // `openshift cli apply`
		if calledApply {
			glog.Errorf("DEPRECATED: The 'apply' command is now deprecated, use 'create' instead.")
		}
		oldRun(c, args)
	}
	return dst
}
