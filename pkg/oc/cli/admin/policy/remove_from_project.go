package policy

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	authorizationutil "github.com/openshift/origin/pkg/authorization/util"
)

const (
	RemoveGroupRecommendedName = "remove-group"
	RemoveUserRecommendedName  = "remove-user"
)

type RemoveFromProjectOptions struct {
	BindingNamespace string
	Client           rbacv1client.RoleBindingsGetter

	Groups []string
	Users  []string

	DryRun bool

	PrintObject func(runtime.Object) error
	Output      string

	genericclioptions.IOStreams
}

func NewRemoveFromProjectOptions(streams genericclioptions.IOStreams) *RemoveFromProjectOptions {
	return &RemoveFromProjectOptions{
		IOStreams: streams,
	}
}

// NewCmdRemoveGroupFromProject implements the OpenShift cli remove-group command
func NewCmdRemoveGroupFromProject(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRemoveFromProjectOptions(streams)
	cmd := &cobra.Command{
		Use:   name + " GROUP [GROUP ...]",
		Short: "Remove group from the current project",
		Long:  `Remove group from the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args, &o.Groups, "group"))
			kcmdutil.CheckErr(o.Validate(f, cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}

	kcmdutil.AddOutputFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)
	return cmd
}

// NewCmdRemoveUserFromProject implements the OpenShift cli remove-user command
func NewCmdRemoveUserFromProject(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRemoveFromProjectOptions(streams)
	cmd := &cobra.Command{
		Use:   name + " USER [USER ...]",
		Short: "Remove user from the current project",
		Long:  `Remove user from the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args, &o.Users, "user"))
			kcmdutil.CheckErr(o.Validate(f, cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}

	kcmdutil.AddPrinterFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)
	return cmd
}

func (o *RemoveFromProjectOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string, target *[]string, targetName string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must specify at least one argument: <%s> [%s]...", targetName, targetName)
	}

	o.Output = kcmdutil.GetFlagString(cmd, "output")
	o.DryRun = kcmdutil.GetFlagBool(cmd, "dry-run")

	*target = append(*target, args...)

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.Client, err = rbacv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	if o.BindingNamespace, _, err = f.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}

	o.PrintObject = func(obj runtime.Object) error {
		return kcmdutil.PrintObject(cmd, obj, o.Out)
	}

	return nil
}

func (o *RemoveFromProjectOptions) Validate(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
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
	sort.Sort(sort.Reverse(roleBindingSorter(roleBindings.Items)))

	usersRemoved := sets.String{}
	groupsRemoved := sets.String{}
	sasRemoved := sets.String{}
	othersRemoved := sets.String{}
	dryRunText := ""
	if o.DryRun {
		dryRunText = " (dry run)"
	}

	updatedBindings := &rbacv1.RoleBindingList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "List",
			APIVersion: "v1",
		},
		ListMeta: metav1.ListMeta{},
	}

	subjectsToRemove := authorizationutil.BuildRBACSubjects(o.Users, o.Groups)

	for _, currBinding := range roleBindings.Items {
		originalSubjects := make([]rbacv1.Subject, len(currBinding.Subjects))
		copy(originalSubjects, currBinding.Subjects)
		oldUsers, oldGroups, oldSAs, oldOthers := rbacv1helpers.SubjectsStrings(originalSubjects)
		oldUsersSet, oldGroupsSet, oldSAsSet, oldOtherSet := sets.NewString(oldUsers...), sets.NewString(oldGroups...), sets.NewString(oldSAs...), sets.NewString(oldOthers...)

		currBinding.Subjects, _ = removeSubjects(currBinding.Subjects, subjectsToRemove)
		newUsers, newGroups, newSAs, newOthers := rbacv1helpers.SubjectsStrings(currBinding.Subjects)
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

		roleDisplayName := fmt.Sprintf("%s/%s", currBinding.Namespace, currBinding.RoleRef.Name)
		if currBinding.RoleRef.Kind == "ClusterRole" {
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

type roleBindingSorter []rbacv1.RoleBinding

func (s roleBindingSorter) Len() int {
	return len(s)
}
func (s roleBindingSorter) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}
func (s roleBindingSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
