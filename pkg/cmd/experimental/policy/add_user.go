package policy

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
)

type addUserOptions struct {
	roleNamespace string
	roleName      string
	clientConfig  clientcmd.ClientConfig

	bindingNamespace string
	userNames        []string
}

func NewCmdAddUser(clientConfig clientcmd.ClientConfig) *cobra.Command {
	options := &addUserOptions{clientConfig: clientConfig}

	cmd := &cobra.Command{
		Use:   "add-user",
		Short: "add user to role",
		Long:  `add user to role`,
		Run: func(cmd *cobra.Command, args []string) {
			if !options.complete(cmd) {
				return
			}

			err := options.run()
			if err != nil {
				fmt.Printf("%v\n", err)
			}
		},
	}

	cmd.Flags().StringVar(&options.bindingNamespace, "namespace", "", "namespace where the role binding is located.  This should be replaced by clientcmd.ClientConfig")
	cmd.Flags().StringVar(&options.roleNamespace, "role-namespace", "master", "namespace where the role is located.")

	return cmd
}

func (o *addUserOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) < 2 {
		cmd.Help()
		return false
	}

	o.roleName = args[0]
	o.userNames = args[1:]
	return true
}

func (o *addUserOptions) run() error {
	clientConfig, err := o.clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	client, err := client.New(clientConfig)
	if err != nil {
		return err
	}

	roleBinding, roleBindingNames, err := getExistingRoleBindingForRole(o.roleNamespace, o.roleName, o.bindingNamespace, client)
	if err != nil {
		return err
	}
	isUpdate := true
	if roleBinding == nil {
		roleBinding = &authorizationapi.RoleBinding{}
		isUpdate = false
	}

	roleBinding.RoleRef.Namespace = o.roleNamespace
	roleBinding.RoleRef.Name = o.roleName

	users := util.StringSet{}
	users.Insert(roleBinding.UserNames...)
	users.Insert(o.userNames...)
	roleBinding.UserNames = users.List()

	if isUpdate {
		_, err = client.RoleBindings(o.bindingNamespace).Update(roleBinding)
	} else {
		roleBinding.Name = getUniqueName(o.roleName, roleBindingNames)
		_, err = client.RoleBindings(o.bindingNamespace).Create(roleBinding)
	}
	if err != nil {
		return err
	}

	return nil
}
