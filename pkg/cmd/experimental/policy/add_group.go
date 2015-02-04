package policy

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
)

type addGroupOptions struct {
	roleNamespace string
	roleName      string
	clientConfig  clientcmd.ClientConfig

	groupNames []string
}

func NewCmdAddGroup(clientConfig clientcmd.ClientConfig) *cobra.Command {
	options := &addGroupOptions{clientConfig: clientConfig}

	cmd := &cobra.Command{
		Use:   "add-group <role> <group> [group]...",
		Short: "add group to role",
		Long:  `add group to role`,
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

	cmd.Flags().StringVar(&options.roleNamespace, "role-namespace", "master", "namespace where the role is located.")

	return cmd
}

func (o *addGroupOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) < 2 {
		cmd.Help()
		return false
	}

	o.roleName = args[0]
	o.groupNames = args[1:]
	return true
}

func (o *addGroupOptions) run() error {
	clientConfig, err := o.clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	client, err := client.New(clientConfig)
	if err != nil {
		return err
	}
	namespace, err := o.clientConfig.Namespace()
	if err != nil {
		return err
	}

	roleBinding, roleBindingNames, err := getExistingRoleBindingForRole(o.roleNamespace, o.roleName, namespace, client)
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

	groups := util.StringSet{}
	groups.Insert(roleBinding.GroupNames...)
	groups.Insert(o.groupNames...)
	roleBinding.GroupNames = groups.List()

	if isUpdate {
		_, err = client.RoleBindings(namespace).Update(roleBinding)
	} else {
		roleBinding.Name = getUniqueName(o.roleName, roleBindingNames)
		_, err = client.RoleBindings(namespace).Create(roleBinding)
	}
	if err != nil {
		return err
	}

	return nil
}
