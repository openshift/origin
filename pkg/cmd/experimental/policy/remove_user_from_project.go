package policy

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/client"
)

type removeUserFromProjectOptions struct {
	clientConfig clientcmd.ClientConfig

	userNames []string
}

func NewCmdRemoveUserFromProject(clientConfig clientcmd.ClientConfig) *cobra.Command {
	options := &removeUserFromProjectOptions{clientConfig: clientConfig}

	cmd := &cobra.Command{
		Use:   "remove-user-from-project  <user> [user]...",
		Short: "remove user from project",
		Long:  `remove user from project`,
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

	return cmd
}

func (o *removeUserFromProjectOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) < 1 {
		cmd.Help()
		return false
	}

	o.userNames = args
	return true
}

func (o *removeUserFromProjectOptions) run() error {
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

	bindingList, err := client.PolicyBindings(namespace).List(klabels.Everything(), klabels.Everything())
	if err != nil {
		return err
	}

	for _, currBindings := range bindingList.Items {
		for _, currBinding := range currBindings.RoleBindings {
			usersForBinding := util.StringSet{}
			usersForBinding.Insert(currBinding.UserNames...)
			usersForBinding.Delete(o.userNames...)

			currBinding.UserNames = usersForBinding.List()

			_, err = client.RoleBindings(namespace).Update(&currBinding)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
