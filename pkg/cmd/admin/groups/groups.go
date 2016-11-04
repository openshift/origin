package groups

import (
	"io"

	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/admin/groups/sync/cli"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const GroupsRecommendedName = "groups"

var groupLong = templates.LongDesc(`
	Manage groups in your cluster

	Groups are sets of users that can be used when describing policy.`)

func NewCmdGroups(name, fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Manage groups",
		Long:  groupLong,
		Run:   cmdutil.DefaultSubCommandRun(errOut),
	}

	cmds.AddCommand(NewCmdNewGroup(NewGroupRecommendedName, fullName+" "+NewGroupRecommendedName, f, out))
	cmds.AddCommand(NewCmdAddUsers(AddRecommendedName, fullName+" "+AddRecommendedName, f, out))
	cmds.AddCommand(NewCmdRemoveUsers(RemoveRecommendedName, fullName+" "+RemoveRecommendedName, f, out))
	cmds.AddCommand(cli.NewCmdSync(cli.SyncRecommendedName, fullName+" "+cli.SyncRecommendedName, f, out))
	cmds.AddCommand(cli.NewCmdPrune(cli.PruneRecommendedName, fullName+" "+cli.PruneRecommendedName, f, out))

	return cmds
}
