package cli

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/version"
)

const longDesc = `
OpenShift Client

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
    https://github.com/openshift/origin for the latest information on OpenShift.
`

func NewCommandCLI(name, fullName string) *cobra.Command {
	// Main command
	cmds := &cobra.Command{
		Use:   name,
		Short: "Client tools for OpenShift",
		Long:  fmt.Sprintf(longDesc, fullName),
		Run: func(c *cobra.Command, args []string) {
			c.SetOutput(os.Stdout)
			c.Help()
		},
	}

	f := clientcmd.New(cmds.PersistentFlags())
	in := os.Stdin
	out := os.Stdout

	templates.UseCliTemplates(cmds)

	cmds.AddCommand(cmd.NewCmdLogin(f, in, out))
	cmds.AddCommand(cmd.NewCmdNewApplication(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdStartBuild(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdCancelBuild(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdBuildLogs(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdRollback(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdGet(fullName, f, out))
	cmds.AddCommand(f.NewCmdDescribe(out))
	// Deprecate 'osc apply' with 'osc create' command.
	cmds.AddCommand(applyToCreate(cmd.NewCmdCreate(fullName, f, out)))
	cmds.AddCommand(cmd.NewCmdProcess(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdUpdate(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdDelete(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdLog(fullName, f, out))
	cmds.AddCommand(cmd.NewCmdExec(fullName, f, os.Stdin, out, os.Stderr))
	cmds.AddCommand(cmd.NewCmdPortForward(fullName, f))
	cmds.AddCommand(f.NewCmdProxy(out))
	cmds.AddCommand(cmd.NewCmdProject(f, out))
	cmds.AddCommand(cmd.NewCmdOptions(f, out))
	cmds.AddCommand(version.NewVersionCommand(fullName))

	return cmds
}

// NewCmdKubectl provides exactly the functionality from Kubernetes,
// but with support for OpenShift resources
func NewCmdKubectl(name string) *cobra.Command {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	f := clientcmd.New(flags)
	cmd := f.Factory.NewKubectlCommand(os.Stdin, os.Stdout, os.Stderr)
	cmd.Aliases = []string{"kubectl"}
	cmd.Use = name
	cmd.Short = "Kubernetes cluster management via kubectl"
	cmd.Long = cmd.Long + "\n\nThis command is provided for direct management of the Kubernetes cluster OpenShift runs on."
	flags.VisitAll(func(flag *pflag.Flag) {
		if f := cmd.PersistentFlags().Lookup(flag.Name); f == nil {
			cmd.PersistentFlags().AddFlag(flag)
		} else {
			glog.V(6).Infof("already registered flag %s", flag.Name)
		}
	})
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
		calledApply = calledApply || len(os.Args) >= 2 && os.Args[1] == "apply" // `osc apply`
		calledApply = calledApply || len(os.Args) >= 3 && os.Args[2] == "apply" // `openshift cli apply`
		if calledApply {
			glog.Errorf("DEPRECATED: The 'apply' command is now deprecated, use 'create' instead.")
		}
		oldRun(c, args)
	}
	return dst
}
