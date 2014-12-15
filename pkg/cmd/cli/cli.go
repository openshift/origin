package cli

import (
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/cli/cmd"
)

const longDesc = `
OpenShift Client

The OpenShift client exposes commands for managing your applications, as well as lower level
tools to interact with each component of your system.

At the present time, the CLI wraps many of the upstream Kubernetes commands and works generically
on all resources.  Some commands you can try:

    $ %[1]s get pods

Note: This is an alpha release of OpenShift and will change significantly.  See
    https://github.com/openshift/origin for the latest information on OpenShift.
`

func NewCommandCLI(name string) *cobra.Command {
	// Main command
	cmds := &cobra.Command{
		Use:     name,
		Aliases: []string{"kubectl"},
		Short:   "Client tools for OpenShift",
		Long:    fmt.Sprintf(longDesc, name),
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	// TODO: there should be two client builders, one for OpenShift, and one for Kubernetes
	clientBuilder := clientcmd.NewBuilder(clientcmd.NewPromptingAuthLoader(os.Stdin))
	clientBuilder.BindFlags(cmds.PersistentFlags())

	cmds.PersistentFlags().String("ns-path", os.Getenv("HOME")+"/.kubernetes_ns", "Path to the namespace info file that holds the name space context to use for CLI requests.")
	cmds.PersistentFlags().StringP("namespace", "n", "", "If present, the namespace scope for this CLI request.")

	f := cmd.NewFactory(clientBuilder)
	out := os.Stdout

	// Kubernetes CRUD commands
	cmds.AddCommand(f.NewCmdGet(out))
	cmds.AddCommand(f.NewCmdDescribe(out))
	cmds.AddCommand(f.NewCmdCreate(out))
	cmds.AddCommand(f.NewCmdCreateAll(out))
	cmds.AddCommand(f.NewCmdUpdate(out))
	cmds.AddCommand(f.NewCmdDelete(out))
	cmds.AddCommand(kubecmd.NewCmdNamespace(out))

	// Kubernetes support commands
	cmds.AddCommand(f.NewCmdLog(out))
	cmds.AddCommand(f.NewCmdProxy(out))

	// Origin commands
	cmds.AddCommand(cmd.NewCmdApply(f.Factory, out))
	cmds.AddCommand(cmd.NewCmdProcess(f.Factory, out))

	// Origin build commands
	cmds.AddCommand(cmd.NewCmdBuildLogs(f.Factory, out))
	cmds.AddCommand(cmd.NewCmdStartBuild(f, out))
	cmds.AddCommand(cmd.NewCmdCancelBuild(f, out))

	return cmds
}
