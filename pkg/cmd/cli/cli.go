package cli

import (
	"fmt"
	"os"

	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const longDesc = `
OpenShift Client

The OpenShift client exposes commands for managing your applications, as well as lower level
tools to interact with each component of your system.

At the present time, the CLI wraps many of the upstream Kubernetes commands and works generically
on all resources.  To create a new application, try:

    $ %[1]s new-app openshift/ruby-20-centos~git@github.com/mfojtik/sinatra-app-example

This will create an application based on the Docker image 'openshift/ruby-20-centos' that builds
the source code at 'github.com/mfojtik/sinatra-app-example'. To start the build, run

    $ %[1]s start-build sinatra-app-example

and watch the build logs and build status with:

    $ %[1]s get builds
    $ %[1]s build-logs <name_of_build>

You'll be able to view the deployed application on the IP and port of the service that new-app
created for you.

Note: This is an alpha release of OpenShift and will change significantly.  See
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
	out := os.Stdout

	cmds.SetUsageTemplate(templates.CliUsageTemplate())
	cmds.SetHelpTemplate(templates.CliHelpTemplate())

	cmds.AddCommand(cmd.NewCmdNewApplication(f, out))
	cmds.AddCommand(cmd.NewCmdStartBuild(f, out))
	cmds.AddCommand(cmd.NewCmdCancelBuild(f, out))
	cmds.AddCommand(cmd.NewCmdBuildLogs(f, out))
	cmds.AddCommand(cmd.NewCmdRollback(name, "rollback", f, out))

	cmds.AddCommand(f.NewCmdGet(out))
	cmds.AddCommand(f.NewCmdDescribe(out))
	// Deprecate 'osc apply' with 'osc create' command.
	cmds.AddCommand(applyToCreate(f.NewCmdCreate(out)))
	cmds.AddCommand(cmd.NewCmdProcess(f, out))
	cmds.AddCommand(f.NewCmdUpdate(out))
	cmds.AddCommand(f.NewCmdDelete(out))

	cmds.AddCommand(f.NewCmdLog(out))
	cmds.AddCommand(f.NewCmdProxy(out))

	cmds.AddCommand(kubecmd.NewCmdNamespace(out))

	// Origin build commands

	cmds.AddCommand(cmd.NewCmdOptions(f, out))

	return cmds
}

// NewCmdKubectl provides exactly the functionality from Kubernetes,
// but with support for OpenShift resources
func NewCmdKubectl(name string) *cobra.Command {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	f := clientcmd.New(flags)
	cmd := f.Factory.NewKubectlCommand(os.Stdout)
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
