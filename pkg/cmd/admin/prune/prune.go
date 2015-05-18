package prune

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const PruneRecommendedName = "prune"

func NewCommandPrune(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Prune resources",
		Long:  `Prune resources`,
		Run:   runHelp,
	}

	cmds.AddCommand(NewCmdPruneBuilds(f, fullName, PruneBuildsRecommendedName, out))
	cmds.AddCommand(NewCmdPruneDeployments(f, fullName, PruneDeploymentsRecommendedName, out))
	return cmds
}

func runHelp(cmd *cobra.Command, args []string) {
	cmd.Help()
}
