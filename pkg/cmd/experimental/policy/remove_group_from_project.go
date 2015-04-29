package policy

import (
	"errors"
	"sort"

	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
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
	sort.Sort(authorizationapi.PolicyBindingSorter(bindingList.Items))

	for _, currPolicyBinding := range bindingList.Items {
		for _, currBinding := range authorizationapi.SortRoleBindings(currPolicyBinding.RoleBindings, true) {
			if !currBinding.Groups.HasAny(o.groups...) {
				continue
			}

			currBinding.Groups.Delete(o.groups...)

			_, err = o.client.RoleBindings(o.bindingNamespace).Update(&currBinding)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
