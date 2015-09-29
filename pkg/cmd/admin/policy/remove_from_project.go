package policy

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	uservalidation "github.com/openshift/origin/pkg/user/api/validation"
)

const (
	RemoveGroupRecommendedName = "remove-group"
	RemoveUserRecommendedName  = "remove-user"
)

type RemoveFromProjectOptions struct {
	BindingNamespace string
	Client           client.Interface

	Groups []string
	Users  []string

	Out io.Writer
}

// NewCmdRemoveGroupFromProject implements the OpenShift cli remove-group command
func NewCmdRemoveGroupFromProject(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RemoveFromProjectOptions{Out: out}

	cmd := &cobra.Command{
		Use:   name + " GROUP [GROUP ...]",
		Short: "Remove group from the current project",
		Long:  `Remove group from the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, &options.Groups, "group"); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.Run(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	return cmd
}

// NewCmdRemoveUserFromProject implements the OpenShift cli remove-user command
func NewCmdRemoveUserFromProject(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RemoveFromProjectOptions{Out: out}

	cmd := &cobra.Command{
		Use:   name + " USER [USER ...]",
		Short: "Remove user from the current project",
		Long:  `Remove user from the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, &options.Users, "user"); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.Run(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	return cmd
}

func (o *RemoveFromProjectOptions) Complete(f *clientcmd.Factory, args []string, target *[]string, targetName string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must specify at least one argument: <%s> [%s]...", targetName, targetName)
	}

	*target = append(*target, args...)

	var err error
	if o.Client, _, err = f.Clients(); err != nil {
		return err
	}
	if o.BindingNamespace, _, err = f.DefaultNamespace(); err != nil {
		return err
	}

	return nil
}

func (o *RemoveFromProjectOptions) Run() error {
	bindingList, err := o.Client.PolicyBindings(o.BindingNamespace).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}
	sort.Sort(authorizationapi.PolicyBindingSorter(bindingList.Items))

	usersRemoved := sets.String{}
	groupsRemoved := sets.String{}
	sasRemoved := sets.String{}
	othersRemoved := sets.String{}

	subjectsToRemove := authorizationapi.BuildSubjects(o.Users, o.Groups, uservalidation.ValidateUserName, uservalidation.ValidateGroupName)

	for _, currPolicyBinding := range bindingList.Items {
		for _, currBinding := range authorizationapi.SortRoleBindings(currPolicyBinding.RoleBindings, true) {
			originalSubjects := make([]kapi.ObjectReference, len(currBinding.Subjects))
			copy(originalSubjects, currBinding.Subjects)
			oldUsers, oldGroups, oldSAs, oldOthers := authorizationapi.SubjectsStrings(currBinding.Namespace, originalSubjects)
			oldUsersSet, oldGroupsSet, oldSAsSet, oldOtherSet := sets.NewString(oldUsers...), sets.NewString(oldGroups...), sets.NewString(oldSAs...), sets.NewString(oldOthers...)

			currBinding.Subjects = removeSubjects(currBinding.Subjects, subjectsToRemove)
			newUsers, newGroups, newSAs, newOthers := authorizationapi.SubjectsStrings(currBinding.Namespace, currBinding.Subjects)
			newUsersSet, newGroupsSet, newSAsSet, newOtherSet := sets.NewString(newUsers...), sets.NewString(newGroups...), sets.NewString(newSAs...), sets.NewString(newOthers...)

			if len(currBinding.Subjects) == len(originalSubjects) {
				continue
			}

			_, err = o.Client.RoleBindings(o.BindingNamespace).Update(currBinding)
			if err != nil {
				return err
			}

			roleDisplayName := fmt.Sprintf("%s/%s", currBinding.RoleRef.Namespace, currBinding.RoleRef.Name)
			if len(currBinding.RoleRef.Namespace) == 0 {
				roleDisplayName = currBinding.RoleRef.Name
			}

			if diff := oldUsersSet.Difference(newUsersSet); len(diff) != 0 {
				fmt.Fprintf(o.Out, "Removing %s from users %v in project %s.\n", roleDisplayName, diff.List(), o.BindingNamespace)
				usersRemoved.Insert(diff.List()...)
			}
			if diff := oldGroupsSet.Difference(newGroupsSet); len(diff) != 0 {
				fmt.Fprintf(o.Out, "Removing %s from groups %v in project %s.\n", roleDisplayName, diff.List(), o.BindingNamespace)
				groupsRemoved.Insert(diff.List()...)
			}
			if diff := oldSAsSet.Difference(newSAsSet); len(diff) != 0 {
				fmt.Fprintf(o.Out, "Removing %s from serviceaccounts %v in project %s.\n", roleDisplayName, diff.List(), o.BindingNamespace)
				sasRemoved.Insert(diff.List()...)
			}
			if diff := oldOtherSet.Difference(newOtherSet); len(diff) != 0 {
				fmt.Fprintf(o.Out, "Removing %s from subjects %v in project %s.\n", roleDisplayName, diff.List(), o.BindingNamespace)
				othersRemoved.Insert(diff.List()...)
			}
		}
	}

	if diff := sets.NewString(o.Users...).Difference(usersRemoved); len(diff) != 0 {
		fmt.Fprintf(o.Out, "Users %v were not bound to roles in project %s.\n", diff.List(), o.BindingNamespace)
	}
	if diff := sets.NewString(o.Groups...).Difference(groupsRemoved); len(diff) != 0 {
		fmt.Fprintf(o.Out, "Groups %v were not bound to roles in project %s.\n", diff.List(), o.BindingNamespace)
	}

	return nil
}
