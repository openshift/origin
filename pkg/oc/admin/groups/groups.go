package groups

import (
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/oc/admin/groups/sync/cli"
)

const GroupsRecommendedName = "groups"

var groupLong = templates.LongDesc(`
	Manage groups in your cluster

	Groups are sets of users that can be used when describing policy.`)

func NewCmdGroups(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Manage groups",
		Long:  groupLong,
		Run:   kcmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	cmds.AddCommand(NewCmdNewGroup(NewGroupRecommendedName, fullName+" "+NewGroupRecommendedName, f, streams))
	cmds.AddCommand(NewCmdAddUsers(AddRecommendedName, fullName+" "+AddRecommendedName, f, streams))
	cmds.AddCommand(NewCmdRemoveUsers(RemoveRecommendedName, fullName+" "+RemoveRecommendedName, f, streams))
	cmds.AddCommand(cli.NewCmdSync(cli.SyncRecommendedName, fullName+" "+cli.SyncRecommendedName, f, streams))
	cmds.AddCommand(cli.NewCmdPrune(cli.PruneRecommendedName, fullName+" "+cli.PruneRecommendedName, f, streams))

	return cmds
}
