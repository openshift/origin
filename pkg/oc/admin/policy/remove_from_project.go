package policy

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	oauthorizationtypedclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const (
	RemoveGroupRecommendedName = "remove-group"
	RemoveUserRecommendedName  = "remove-user"
)

type RemoveFromProjectOptions struct {
	BindingNamespace string
	Client           oauthorizationtypedclient.RoleBindingsGetter

	Groups []string
	Users  []string

	DryRun bool

	PrintObject func(runtime.Object) error
	Output      string
	Out         io.Writer
}

// NewCmdRemoveGroupFromProject implements the OpenShift cli remove-group command
func NewCmdRemoveGroupFromProject(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RemoveFromProjectOptions{Out: out}

	cmd := &cobra.Command{
		Use:   name + " GROUP [GROUP ...]",
		Short: "Remove group from the current project",
		Long:  `Remove group from the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, cmd, args, &options.Groups, "group"); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.Validate(f, cmd, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.Run(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	kcmdutil.AddOutputFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)
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
			if err := options.Complete(f, cmd, args, &options.Users, "user"); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.Validate(f, cmd, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.Run(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	kcmdutil.AddPrinterFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)
	return cmd
}

func (o *RemoveFromProjectOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, target *[]string, targetName string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must specify at least one argument: <%s> [%s]...", targetName, targetName)
	}

	o.Output = kcmdutil.GetFlagString(cmd, "output")
	o.DryRun = kcmdutil.GetFlagBool(cmd, "dry-run")

	*target = append(*target, args...)

	authorizationClient, err := f.OpenshiftInternalAuthorizationClient()
	if err != nil {
		return err
	}
	o.Client = authorizationClient.Authorization()
	if o.BindingNamespace, _, err = f.DefaultNamespace(); err != nil {
		return err
	}

	mapper, _ := f.Object()

	o.PrintObject = func(obj runtime.Object) error {
		return f.PrintObject(cmd, false, mapper, obj, o.Out)
	}

	return nil
}

func (o *RemoveFromProjectOptions) Validate(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	if len(o.Output) > 0 && o.Output != "yaml" && o.Output != "json" {
		return fmt.Errorf("invalid output format %q, only yaml|json supported", o.Output)
	}

	return nil
}

func (o *RemoveFromProjectOptions) Run() error {
	roleBindings, err := o.Client.RoleBindings(o.BindingNamespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	// maintain David's hack from #1973 (see #1975, #1976 and https://bugzilla.redhat.com/show_bug.cgi?id=1215969)
	sort.Sort(sort.Reverse(authorizationapi.RoleBindingSorter(roleBindings.Items)))

	usersRemoved := sets.String{}
	groupsRemoved := sets.String{}
	sasRemoved := sets.String{}
	othersRemoved := sets.String{}
	dryRunText := ""
	if o.DryRun {
		dryRunText = " (dry run)"
	}

	updatedBindings := &authorizationapi.RoleBindingList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "List",
			APIVersion: "v1",
		},
		ListMeta: metav1.ListMeta{},
	}

	subjectsToRemove := authorizationapi.BuildSubjects(o.Users, o.Groups)

	for _, currBinding := range roleBindings.Items {
		originalSubjects := make([]kapi.ObjectReference, len(currBinding.Subjects))
		copy(originalSubjects, currBinding.Subjects)
		oldUsers, oldGroups, oldSAs, oldOthers := authorizationapi.SubjectsStrings(currBinding.Namespace, originalSubjects)
		oldUsersSet, oldGroupsSet, oldSAsSet, oldOtherSet := sets.NewString(oldUsers...), sets.NewString(oldGroups...), sets.NewString(oldSAs...), sets.NewString(oldOthers...)

		currBinding.Subjects, _ = removeSubjects(currBinding.Subjects, subjectsToRemove)
		newUsers, newGroups, newSAs, newOthers := authorizationapi.SubjectsStrings(currBinding.Namespace, currBinding.Subjects)
		newUsersSet, newGroupsSet, newSAsSet, newOtherSet := sets.NewString(newUsers...), sets.NewString(newGroups...), sets.NewString(newSAs...), sets.NewString(newOthers...)

		if len(currBinding.Subjects) == len(originalSubjects) {
			continue
		}

		if len(o.Output) > 0 {
			updatedBindings.Items = append(updatedBindings.Items, currBinding)
			continue
		}

		if !o.DryRun {
			if len(currBinding.Subjects) > 0 {
				_, err = o.Client.RoleBindings(o.BindingNamespace).Update(&currBinding)
			} else {
				err = o.Client.RoleBindings(o.BindingNamespace).Delete(currBinding.Name, &metav1.DeleteOptions{})
			}
			if err != nil {
				return err
			}
		}

		roleDisplayName := fmt.Sprintf("%s/%s", currBinding.RoleRef.Namespace, currBinding.RoleRef.Name)
		if len(currBinding.RoleRef.Namespace) == 0 {
			roleDisplayName = currBinding.RoleRef.Name
		}

		if diff := oldUsersSet.Difference(newUsersSet); len(diff) != 0 {
			fmt.Fprintf(o.Out, "Removing %s from users %v in project %s%s.\n", roleDisplayName, diff.List(), o.BindingNamespace, dryRunText)
			usersRemoved.Insert(diff.List()...)
		}
		if diff := oldGroupsSet.Difference(newGroupsSet); len(diff) != 0 {
			fmt.Fprintf(o.Out, "Removing %s from groups %v in project %s%s.\n", roleDisplayName, diff.List(), o.BindingNamespace, dryRunText)
			groupsRemoved.Insert(diff.List()...)
		}
		if diff := oldSAsSet.Difference(newSAsSet); len(diff) != 0 {
			fmt.Fprintf(o.Out, "Removing %s from serviceaccounts %v in project %s%s.\n", roleDisplayName, diff.List(), o.BindingNamespace, dryRunText)
			sasRemoved.Insert(diff.List()...)
		}
		if diff := oldOtherSet.Difference(newOtherSet); len(diff) != 0 {
			fmt.Fprintf(o.Out, "Removing %s from subjects %v in project %s%s.\n", roleDisplayName, diff.List(), o.BindingNamespace, dryRunText)
			othersRemoved.Insert(diff.List()...)
		}
	}

	if len(o.Output) > 0 {
		return o.PrintObject(updatedBindings)
	}

	if diff := sets.NewString(o.Users...).Difference(usersRemoved); len(diff) != 0 {
		fmt.Fprintf(o.Out, "Users %v were not bound to roles in project %s%s.\n", diff.List(), o.BindingNamespace, dryRunText)
	}
	if diff := sets.NewString(o.Groups...).Difference(groupsRemoved); len(diff) != 0 {
		fmt.Fprintf(o.Out, "Groups %v were not bound to roles in project %s%s.\n", diff.List(), o.BindingNamespace, dryRunText)
	}

	return nil
}
