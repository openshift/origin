package users

import (
	"fmt"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"
)

const RemoveRecommendedName = "remove-users"

var (
	removeLong = templates.LongDesc(`
		Remove users from a group.

		This command will remove users from the list of members for a group.`)

	removeExample = templates.Examples(`
		# Remove user1 and user2 from my-group
	%[1]s my-group user1 user2`)
)

type RemoveUsersOptions struct {
	GroupModificationOptions *GroupModificationOptions
}

func NewCmdRemoveUsers(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &RemoveUsersOptions{
		GroupModificationOptions: NewGroupModificationOptions(streams),
	}
	cmd := &cobra.Command{
		Use:     name + " GROUP USER [USER ...]",
		Short:   "Remove users from a group",
		Long:    removeLong,
		Example: fmt.Sprintf(removeExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}
	o.GroupModificationOptions.PrintFlags.AddFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *RemoveUsersOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	return o.GroupModificationOptions.Complete(f, cmd, args)
}

func (o *RemoveUsersOptions) Run() error {
	group, err := o.GroupModificationOptions.GroupClient.Groups().Get(o.GroupModificationOptions.Group, metav1.GetOptions{})
	if err != nil {
		return err
	}

	toDelete := sets.NewString(o.GroupModificationOptions.Users...)
	newUsers := []string{}
	for _, user := range group.Users {
		if toDelete.Has(user) {
			continue
		}

		newUsers = append(newUsers, user)
	}
	group.Users = newUsers

	if !o.GroupModificationOptions.DryRun {
		group, err = o.GroupModificationOptions.GroupClient.Groups().Update(group)
		if err != nil {
			return err
		}
	}

	return o.GroupModificationOptions.PrintObj("removed", group)
}
