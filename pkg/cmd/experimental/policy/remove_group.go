package policy

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/client"
)

type removeGroupOptions struct {
	roleNamespace string
	roleName      string
	clientConfig  clientcmd.ClientConfig

	groupNames []string
}

func NewCmdRemoveGroup(clientConfig clientcmd.ClientConfig) *cobra.Command {
	options := &removeGroupOptions{clientConfig: clientConfig}

	cmd := &cobra.Command{
		Use:   "remove-group <role> <group> [group]...",
		Short: "remove group from role",
		Long:  `remove group from role`,
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

func (o *removeGroupOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) < 2 {
		cmd.Help()
		return false
	}

	o.roleName = args[0]
	o.groupNames = args[1:]
	return true
}

func (o *removeGroupOptions) run() error {
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

	groups := util.StringSet{}
	groups.Insert(roleBinding.GroupNames...)
	groups.Delete(o.groupNames...)
	roleBinding.GroupNames = groups.List()

	_, err = client.RoleBindings(namespace).Update(roleBinding)
	if err != nil {
		return err
	}

	return nil
}
