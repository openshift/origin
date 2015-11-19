package network

import (
	"io"

	"github.com/spf13/cobra"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const PodNetworkCommandName = "pod-network"

const (
	podNetworkLong = `
Manage pod network in the cluster

This command provides common pod network operations for administrators.`
)

func NewCmdPodNetwork(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Manage pod network",
		Long:  podNetworkLong,
		Run:   cmdutil.DefaultSubCommandRun(out),
	}

	cmds.AddCommand(NewCmdJoinProjectsNetwork(JoinProjectsNetworkCommandName, fullName+" "+JoinProjectsNetworkCommandName, f, out))
	cmds.AddCommand(NewCmdMakeGlobalProjectsNetwork(MakeGlobalProjectsNetworkCommandName, fullName+" "+MakeGlobalProjectsNetworkCommandName, f, out))

	// TODO: Enable isolate-projects subcommand once we move VNID allocation to REST layer
	//cmds.AddCommand(NewCmdIsolateProjectsNetwork(IsolateProjectsNetworkCommandName, fullName+" "+IsolateProjectsNetworkCommandName, f, out))

	return cmds
}
