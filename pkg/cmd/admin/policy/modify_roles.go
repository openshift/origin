package policy

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	uservalidation "github.com/openshift/origin/pkg/user/api/validation"
)

const (
	AddRoleToGroupRecommendedName      = "add-role-to-group"
	AddRoleToUserRecommendedName       = "add-role-to-user"
	RemoveRoleFromGroupRecommendedName = "remove-role-from-group"
	RemoveRoleFromUserRecommendedName  = "remove-role-from-user"

	AddClusterRoleToGroupRecommendedName      = "add-cluster-role-to-group"
	AddClusterRoleToUserRecommendedName       = "add-cluster-role-to-user"
	RemoveClusterRoleFromGroupRecommendedName = "remove-cluster-role-from-group"
	RemoveClusterRoleFromUserRecommendedName  = "remove-cluster-role-from-user"
)

type RoleModificationOptions struct {
	RoleNamespace       string
	RoleName            string
	RoleBindingAccessor RoleBindingAccessor

	Users    []string
	Groups   []string
	Subjects []kapi.ObjectReference
}

// NewCmdAddRoleToGroup implements the OpenShift cli add-role-to-group command
func NewCmdAddRoleToGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}

	cmd := &cobra.Command{
		Use:   name + " ROLE GROUP [GROUP ...]",
		Short: "Add groups to a role in the current project",
		Long:  `Add groups to a role in the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, &options.Groups, "group", true); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.AddRole(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleNamespace, "role-namespace", "", "namespace where the role is located: empty means a role defined in cluster policy")

	return cmd
}

// NewCmdAddRoleToUser implements the OpenShift cli add-role-to-user command
func NewCmdAddRoleToUser(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}
	saNames := util.StringList{}

	cmd := &cobra.Command{
		Use:   name + " ROLE USER [USER ...]",
		Short: "Add users to a role in the current project",
		Long:  `Add users to a role in the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.CompleteUserWithSA(f, args, saNames); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.AddRole(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleNamespace, "role-namespace", "", "namespace where the role is located: empty means a role defined in cluster policy")
	cmd.Flags().VarP(&saNames, "serviceaccount", "z", "service account in the current namespace to use as a user")

	return cmd
}

// NewCmdRemoveRoleFromGroup implements the OpenShift cli remove-role-from-group command
func NewCmdRemoveRoleFromGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}

	cmd := &cobra.Command{
		Use:   name + " ROLE GROUP [GROUP ...]",
		Short: "Remove group from role in the current project",
		Long:  `Remove group from role in the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, &options.Groups, "group", true); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.RemoveRole(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleNamespace, "role-namespace", "", "namespace where the role is located: empty means a role defined in cluster policy")

	return cmd
}

// NewCmdRemoveRoleFromUser implements the OpenShift cli remove-role-from-user command
func NewCmdRemoveRoleFromUser(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}
	saNames := util.StringList{}

	cmd := &cobra.Command{
		Use:   name + " ROLE USER [USER ...]",
		Short: "Remove user from role in the current project",
		Long:  `Remove user from role in the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.CompleteUserWithSA(f, args, saNames); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.RemoveRole(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleNamespace, "role-namespace", "", "namespace where the role is located: empty means a role defined in cluster policy")
	cmd.Flags().VarP(&saNames, "serviceaccount", "z", "service account in the current namespace to use as a user")

	return cmd
}

// NewCmdAddClusterRoleToGroup implements the OpenShift cli add-cluster-role-to-group command
func NewCmdAddClusterRoleToGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}

	cmd := &cobra.Command{
		Use:   name + " <role> <group> [group]...",
		Short: "Add groups to a role for all projects in the cluster",
		Long:  `Add groups to a role for all projects in the cluster`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, &options.Groups, "group", false); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.AddRole(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	return cmd
}

// NewCmdAddClusterRoleToUser implements the OpenShift cli add-cluster-role-to-user command
func NewCmdAddClusterRoleToUser(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}

	cmd := &cobra.Command{
		Use:   name + " <role> <user> [user]...",
		Short: "add users to a role for all projects in the cluster",
		Long:  `add users to a role for all projects in the cluster`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, &options.Users, "user", false); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.AddRole(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	return cmd
}

// NewCmdRemoveClusterRoleFromGroup implements the OpenShift cli remove-cluster-role-from-group command
func NewCmdRemoveClusterRoleFromGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}

	cmd := &cobra.Command{
		Use:   name + " <role> <group> [group]...",
		Short: "remove group from role for all projects in the cluster",
		Long:  `remove group from role for all projects in the cluster`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, &options.Groups, "group", false); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.RemoveRole(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	return cmd
}

// NewCmdRemoveClusterRoleFromUser implements the OpenShift cli remove-cluster-role-from-user command
func NewCmdRemoveClusterRoleFromUser(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}

	cmd := &cobra.Command{
		Use:   name + " <role> <user> [user]...",
		Short: "remove user from role for all projects in the cluster",
		Long:  `remove user from role for all projects in the cluster`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, &options.Users, "user", false); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.RemoveRole(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	return cmd
}

func (o *RoleModificationOptions) CompleteUserWithSA(f *clientcmd.Factory, args []string, saNames util.StringList) error {
	if (len(args) < 2) && (len(saNames) == 0) {
		return errors.New("You must specify at least two arguments: <role> <user> [user]...")
	}

	o.RoleName = args[0]
	if len(args) > 1 {
		o.Users = append(o.Users, args[1:]...)
	}

	osClient, _, err := f.Clients()
	if err != nil {
		return err
	}

	roleBindingNamespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.RoleBindingAccessor = NewLocalRoleBindingAccessor(roleBindingNamespace, osClient)

	for _, sa := range saNames {
		o.Subjects = append(o.Subjects, kapi.ObjectReference{Name: sa, Kind: "ServiceAccount"})
	}

	return nil
}

func (o *RoleModificationOptions) Complete(f *clientcmd.Factory, args []string, target *[]string, targetName string, isNamespaced bool) error {
	if len(args) < 2 {
		return fmt.Errorf("You must specify at least two arguments: <role> <%s> [%s]...", targetName, targetName)
	}

	o.RoleName = args[0]
	*target = append(*target, args[1:]...)

	osClient, _, err := f.Clients()
	if err != nil {
		return err
	}

	if isNamespaced {
		roleBindingNamespace, _, err := f.DefaultNamespace()
		if err != nil {
			return err
		}
		o.RoleBindingAccessor = NewLocalRoleBindingAccessor(roleBindingNamespace, osClient)

	} else {
		o.RoleBindingAccessor = NewClusterRoleBindingAccessor(osClient)

	}

	return nil
}

func (o *RoleModificationOptions) AddRole() error {
	roleBindings, err := o.RoleBindingAccessor.GetExistingRoleBindingsForRole(o.RoleNamespace, o.RoleName)
	if err != nil {
		return err
	}
	roleBindingNames, err := o.RoleBindingAccessor.GetExistingRoleBindingNames()
	if err != nil {
		return err
	}

	var roleBinding *authorizationapi.RoleBinding
	isUpdate := true
	if len(roleBindings) == 0 {
		roleBinding = &authorizationapi.RoleBinding{}
		isUpdate = false
	} else {
		// only need to add the user or group to a single roleBinding on the role.  Just choose the first one
		roleBinding = roleBindings[0]
	}

	roleBinding.RoleRef.Namespace = o.RoleNamespace
	roleBinding.RoleRef.Name = o.RoleName

	newSubjects := authorizationapi.BuildSubjects(o.Users, o.Groups, uservalidation.ValidateUserName, uservalidation.ValidateGroupName)
	newSubjects = append(newSubjects, o.Subjects...)

subjectCheck:
	for _, newSubject := range newSubjects {
		for _, existingSubject := range roleBinding.Subjects {
			if existingSubject.Kind == newSubject.Kind &&
				existingSubject.Name == newSubject.Name &&
				existingSubject.Namespace == newSubject.Namespace {
				continue subjectCheck
			}
		}

		roleBinding.Subjects = append(roleBinding.Subjects, newSubject)
	}

	if isUpdate {
		err = o.RoleBindingAccessor.UpdateRoleBinding(roleBinding)
	} else {
		roleBinding.Name = getUniqueName(o.RoleName, roleBindingNames)
		err = o.RoleBindingAccessor.CreateRoleBinding(roleBinding)
	}
	if err != nil {
		return err
	}

	return nil
}

func (o *RoleModificationOptions) RemoveRole() error {
	roleBindings, err := o.RoleBindingAccessor.GetExistingRoleBindingsForRole(o.RoleNamespace, o.RoleName)
	if err != nil {
		return err
	}
	if len(roleBindings) == 0 {
		return fmt.Errorf("unable to locate RoleBinding for %v/%v", o.RoleNamespace, o.RoleName)
	}

	subjectsToRemove := authorizationapi.BuildSubjects(o.Users, o.Groups, uservalidation.ValidateUserName, uservalidation.ValidateGroupName)
	subjectsToRemove = append(subjectsToRemove, o.Subjects...)

	for _, roleBinding := range roleBindings {
		roleBinding.Subjects = removeSubjects(roleBinding.Subjects, subjectsToRemove)

		err = o.RoleBindingAccessor.UpdateRoleBinding(roleBinding)
		if err != nil {
			return err
		}
	}

	return nil
}

func removeSubjects(haystack, needles []kapi.ObjectReference) []kapi.ObjectReference {
	newSubjects := []kapi.ObjectReference{}

existingLoop:
	for _, existingSubject := range haystack {
		for _, toRemove := range needles {
			if existingSubject.Kind == toRemove.Kind &&
				existingSubject.Name == toRemove.Name &&
				existingSubject.Namespace == toRemove.Namespace {
				continue existingLoop

			}
		}

		newSubjects = append(newSubjects, existingSubject)
	}

	return newSubjects
}
