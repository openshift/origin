package prune

import (
	"io"

	"github.com/spf13/cobra"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const PruneRecommendedName = "prune"

const pruneLong = `Remove older versions of resources from the server

The commands here allow administrators to manage the older versions of resources on
the system by removing them.`

func NewCommandPrune(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Remove older versions of resources from the server",
		Long:  pruneLong,
		Run:   cmdutil.DefaultSubCommandRun(out),
	}

	cmds.AddCommand(NewCmdPruneBuilds(f, fullName, PruneBuildsRecommendedName, out))
	cmds.AddCommand(NewCmdPruneDeployments(f, fullName, PruneDeploymentsRecommendedName, out))
	cmds.AddCommand(NewCmdPruneImages(f, fullName, PruneImagesRecommendedName, out))
	return cmds
}
