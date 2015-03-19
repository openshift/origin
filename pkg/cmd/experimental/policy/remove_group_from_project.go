package policy

import (
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type removeGroupFromProjectOptions struct {
	bindingNamespace string
	client           client.Interface

	groups []string
}

func NewCmdRemoveGroupFromProject(f *clientcmd.Factory) *cobra.Command {
	options := &removeGroupFromProjectOptions{}

	cmd := &cobra.Command{
		Use:   "remove-group-from-project  <group> [group]...",
		Short: "remove group from project",
		Long:  `remove group from project`,
		Run: func(cmd *cobra.Command, args []string) {
			if !options.complete(cmd) {
				return
			}

			var err error
			if options.client, _, err = f.Clients(cmd); err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}
			if options.bindingNamespace, err = f.DefaultNamespace(cmd); err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}

			if err := options.run(); err != nil {
				glog.Fatal(err)
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

	o.groups = args
	return true
}

func (o *removeGroupFromProjectOptions) run() error {
	bindingList, err := o.client.PolicyBindings(o.bindingNamespace).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}

	for _, currBindings := range bindingList.Items {
		for _, currBinding := range currBindings.RoleBindings {
			currBinding.Groups.Delete(o.groups...)

			_, err = o.client.RoleBindings(o.bindingNamespace).Update(&currBinding)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
