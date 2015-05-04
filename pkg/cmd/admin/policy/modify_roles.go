package policy

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	AddRoleToGroupRecommendedName      = "add-role-to-group"
	AddRoleToUserRecommendedName       = "add-role-to-user"
	RemoveRoleFromGroupRecommendedName = "remove-role-from-group"
	RemoveRoleFromUserRecommendedName  = "remove-role-from-user"
)

type RoleModificationOptions struct {
	RoleNamespace    string
	RoleName         string
	BindingNamespace string
	Client           client.Interface

	Users  []string
	Groups []string
}

func NewCmdAddRoleToGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}

	cmd := &cobra.Command{
		Use:   name + " <role> <group> [group]...",
		Short: "add groups to a role in the current project",
		Long:  `add groups to a role in the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, &options.Groups, "group"); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.AddRole(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleNamespace, "role-namespace", bootstrappolicy.DefaultMasterAuthorizationNamespace, "namespace where the role is located.")

	return cmd
}

func NewCmdAddRoleToUser(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}

	cmd := &cobra.Command{
		Use:   name + " <role> <user> [user]...",
		Short: "add users to a role in the current project",
		Long:  `add users to a role in the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, &options.Users, "user"); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.AddRole(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleNamespace, "role-namespace", bootstrappolicy.DefaultMasterAuthorizationNamespace, "namespace where the role is located.")

	return cmd
}

func NewCmdRemoveRoleFromGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}

	cmd := &cobra.Command{
		Use:   name + " <role> <group> [group]...",
		Short: "remove group from role in the current project",
		Long:  `remove group from role in the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, &options.Groups, "group"); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.RemoveRole(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleNamespace, "role-namespace", bootstrappolicy.DefaultMasterAuthorizationNamespace, "namespace where the role is located.")

	return cmd
}

func NewCmdRemoveRoleFromUser(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}

	cmd := &cobra.Command{
		Use:   name + " <role> <user> [user]...",
		Short: "remove user from role in the current project",
		Long:  `remove user from role in the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, &options.Users, "user"); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.RemoveRole(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleNamespace, "role-namespace", bootstrappolicy.DefaultMasterAuthorizationNamespace, "namespace where the role is located.")

	return cmd
}

func (o *RoleModificationOptions) Complete(f *clientcmd.Factory, args []string, target *[]string, targetName string) error {
	if len(args) < 2 {
		return fmt.Errorf("You must specify at least two arguments: <role> <%s> [%s]...", targetName, targetName)
	}

	o.RoleName = args[0]
	*target = append(*target, args[1:]...)

	var err error
	if o.Client, _, err = f.Clients(); err != nil {
		return err
	}
	if o.BindingNamespace, err = f.DefaultNamespace(); err != nil {
		return err
	}

	return nil
}

func (o *RoleModificationOptions) AddRole() error {
	roleBindings, err := getExistingRoleBindingsForRole(o.RoleNamespace, o.RoleName, o.Client.PolicyBindings(o.BindingNamespace))
	if err != nil {
		return err
	}
	roleBindingNames, err := getExistingRoleBindingNames(o.Client.PolicyBindings(o.BindingNamespace))
	if err != nil {
		return err
	}

	var roleBinding *authorizationapi.RoleBinding
	isUpdate := true
	if len(roleBindings) == 0 {
		roleBinding = &authorizationapi.RoleBinding{Users: util.NewStringSet(), Groups: util.NewStringSet()}
		isUpdate = false
	} else {
		// only need to add the user or group to a single roleBinding on the role.  Just choose the first one
		roleBinding = roleBindings[0]
	}

	roleBinding.RoleRef.Namespace = o.RoleNamespace
	roleBinding.RoleRef.Name = o.RoleName

	roleBinding.Users.Insert(o.Users...)
	roleBinding.Groups.Insert(o.Groups...)

	if isUpdate {
		_, err = o.Client.RoleBindings(o.BindingNamespace).Update(roleBinding)
	} else {
		roleBinding.Name = getUniqueName(o.RoleName, roleBindingNames)
		_, err = o.Client.RoleBindings(o.BindingNamespace).Create(roleBinding)
	}
	if err != nil {
		return err
	}

	return nil
}

func (o *RoleModificationOptions) RemoveRole() error {
	roleBindings, err := getExistingRoleBindingsForRole(o.RoleNamespace, o.RoleName, o.Client.PolicyBindings(o.BindingNamespace))
	if err != nil {
		return err
	}
	if len(roleBindings) == 0 {
		return fmt.Errorf("unable to locate RoleBinding for %v::%v in %v", o.RoleNamespace, o.RoleName, o.BindingNamespace)
	}

	for _, roleBinding := range roleBindings {
		roleBinding.Groups.Delete(o.Groups...)
		roleBinding.Users.Delete(o.Users...)

		_, err = o.Client.RoleBindings(o.BindingNamespace).Update(roleBinding)
		if err != nil {
			return err
		}
	}

	return nil
}
