package policy

import (
	"github.com/golang/glog"
	"github.com/spf13/cobra"

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
		Use:   "add-user <role> <user> [user]...",
		Short: "add user to role",
		Long:  `add user to role`,
		Run: func(cmd *cobra.Command, args []string) {
			if !options.complete(cmd) {
				return
			}

			var err error
			if options.Client, _, err = f.Clients(); err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}
			if options.BindingNamespace, err = f.DefaultNamespace(); err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}
			if err := options.Run(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleNamespace, "role-namespace", bootstrappolicy.DefaultMasterAuthorizationNamespace, "namespace where the role is located.")

	return cmd
}

func (o *AddUserOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) < 2 {
		cmd.Help()
		return false
	}

	o.RoleName = args[0]
	o.Users = args[1:]
	return true
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
