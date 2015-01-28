package policy

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/client"
)

type removeUserOptions struct {
	roleNamespace string
	roleName      string
	clientConfig  clientcmd.ClientConfig

	userNames []string
}

func NewCmdRemoveUser(clientConfig clientcmd.ClientConfig) *cobra.Command {
	options := &removeUserOptions{clientConfig: clientConfig}

	cmd := &cobra.Command{
		Use:   "remove-user <role> <user> [user]...",
		Short: "remove user from role",
		Long:  `remove user from role`,
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

func (o *removeUserOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) < 2 {
		cmd.Help()
		return false
	}

	o.roleName = args[0]
	o.userNames = args[1:]
	return true
}

func (o *removeUserOptions) run() error {
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

	roleBinding, _, err := getExistingRoleBindingForRole(o.roleNamespace, o.roleName, namespace, client)
	if err != nil {
		return err
	}
	if roleBinding == nil {
		return fmt.Errorf("unable to locate RoleBinding for %v::%v in %v", o.roleNamespace, o.roleName, namespace)
	}

	users := util.StringSet{}
	users.Insert(roleBinding.UserNames...)
	users.Delete(o.userNames...)
	roleBinding.UserNames = users.List()

	_, err = client.RoleBindings(namespace).Update(roleBinding)
	if err != nil {
		return err
	}

	return nil
}
