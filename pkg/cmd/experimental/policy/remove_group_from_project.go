package policy

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/client"
)

type removeGroupFromProjectOptions struct {
	clientConfig clientcmd.ClientConfig

	groupNames []string
}

func NewCmdRemoveGroupFromProject(clientConfig clientcmd.ClientConfig) *cobra.Command {
	options := &removeGroupFromProjectOptions{clientConfig: clientConfig}

	cmd := &cobra.Command{
		Use:   "remove-group-from-project  <group> [group]...",
		Short: "remove group from project",
		Long:  `remove group from project`,
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

func (o *removeGroupFromProjectOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) < 1 {
		cmd.Help()
		return false
	}

	o.groupNames = args
	return true
}

func (o *removeGroupFromProjectOptions) run() error {
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
			groupsForBinding := util.StringSet{}
			groupsForBinding.Insert(currBinding.GroupNames...)
			groupsForBinding.Delete(o.groupNames...)

			currBinding.GroupNames = groupsForBinding.List()

			_, err = client.RoleBindings(namespace).Update(&currBinding)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
