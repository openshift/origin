package policy

import (
	"errors"

	"github.com/spf13/cobra"

	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type AddUserOptions struct {
	RoleNamespace    string
	RoleName         string
	BindingNamespace string
	Client           client.Interface

	Users []string
}

func NewCmdAddUser(f *clientcmd.Factory) *cobra.Command {
	options := &AddUserOptions{}

	cmd := &cobra.Command{
		Use:   "add-role-to-user <role> <user> [user]...",
		Short: "add users to a role",
		Long:  `add users to a role`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.complete(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			var err error
			if options.Client, _, err = f.Clients(); err != nil {
				kcmdutil.CheckErr(err)
			}
			if options.BindingNamespace, err = f.DefaultNamespace(); err != nil {
				kcmdutil.CheckErr(err)
			}
			if err := options.Run(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleNamespace, "role-namespace", bootstrappolicy.DefaultMasterAuthorizationNamespace, "namespace where the role is located.")

	return cmd
}

func (o *AddUserOptions) complete(args []string) error {
	if len(args) < 2 {
		return errors.New("You must specify at least two arguments: <role> <user> [user]...")
	}

	o.RoleName = args[0]
	o.Users = args[1:]
	return nil
}

func (o *AddUserOptions) Run() error {
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
		roleBinding = &authorizationapi.RoleBinding{Users: util.NewStringSet()}
		isUpdate = false
	} else {
		// only need to add the user or group to a single roleBinding on the role.  Just choose the first one
		roleBinding = roleBindings[0]
	}

	roleBinding.RoleRef.Namespace = o.RoleNamespace
	roleBinding.RoleRef.Name = o.RoleName

	roleBinding.Users.Insert(o.Users...)

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
