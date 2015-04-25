package policy

import (
	"errors"

	"github.com/spf13/cobra"

	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"

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
		Use:   "remove-group  <group> [group]...",
		Short: "remove group from project",
		Long:  `remove group from project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.complete(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			var err error
			if options.client, _, err = f.Clients(); err != nil {
				kcmdutil.CheckErr(err)
			}
			if options.bindingNamespace, err = f.DefaultNamespace(); err != nil {
				kcmdutil.CheckErr(err)
			}

			if err := options.run(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	return cmd
}

func (o *removeGroupFromProjectOptions) complete(args []string) error {
	if len(args) < 1 {
		return errors.New("You must specify at least one argument: <group> [group]...")
	}

	o.groups = args
	return nil
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
