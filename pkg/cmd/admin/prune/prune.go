package prune

import (
	"io"

	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	groups "github.com/openshift/origin/pkg/cmd/admin/groups/sync/cli"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	PruneRecommendedName       = "prune"
	PruneGroupsRecommendedName = "groups"
)

var pruneLong = templates.LongDesc(`
	Remove older versions of resources from the server

	The commands here allow administrators to manage the older versions of resources on
	the system by removing them.`)

func NewCommandPrune(name, fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Remove older versions of resources from the server",
		Long:  pruneLong,
		Run:   cmdutil.DefaultSubCommandRun(errOut),
	}

	cmds.AddCommand(NewCmdPruneBuilds(f, fullName, PruneBuildsRecommendedName, out))
	cmds.AddCommand(NewCmdPruneDeployments(f, fullName, PruneDeploymentsRecommendedName, out))
	cmds.AddCommand(NewCmdPruneImages(f, fullName, PruneImagesRecommendedName, out))
	cmds.AddCommand(groups.NewCmdPrune(PruneGroupsRecommendedName, fullName+" "+PruneGroupsRecommendedName, f, out))
	return cmds
}
