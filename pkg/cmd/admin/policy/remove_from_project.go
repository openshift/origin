package policy

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	RemoveGroupRecommendedName = "remove-group"
	RemoveUserRecommendedName  = "remove-user"
)

type RemoveFromProjectOptions struct {
	BindingNamespace string
	Client           client.Interface

	Groups []string
	Users  []string
}

func NewCmdRemoveGroupFromProject(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RemoveFromProjectOptions{}

	cmd := &cobra.Command{
		Use:   name + " GROUP [GROUP ...]",
		Short: "Remove group from the current project",
		Long:  `Remove group from the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, &options.Groups, "group"); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.Run(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	return cmd
}

func NewCmdRemoveUserFromProject(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RemoveFromProjectOptions{}

	cmd := &cobra.Command{
		Use:   name + " USER [USER ...]",
		Short: "Remove user from the current project",
		Long:  `Remove user from the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, &options.Users, "user"); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.Run(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	return cmd
}

func (o *RemoveFromProjectOptions) Complete(f *clientcmd.Factory, args []string, target *[]string, targetName string) error {
	if len(args) < 1 {
		return fmt.Errorf("You must specify at least one argument: <%s> [%s]...", targetName, targetName)
	}

	*target = append(*target, args...)

	var err error
	if o.Client, _, err = f.Clients(); err != nil {
		return err
	}
	if o.BindingNamespace, err = f.DefaultNamespace(); err != nil {
		return err
	}

	return nil
}

func (o *RemoveFromProjectOptions) Run() error {
	bindingList, err := o.Client.PolicyBindings(o.BindingNamespace).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}
	sort.Sort(authorizationapi.PolicyBindingSorter(bindingList.Items))

	for _, currPolicyBinding := range bindingList.Items {
		for _, currBinding := range authorizationapi.SortRoleBindings(currPolicyBinding.RoleBindings, true) {
			if !currBinding.Groups.HasAny(o.Groups...) && !currBinding.Users.HasAny(o.Users...) {
				continue
			}

			currBinding.Groups.Delete(o.Groups...)
			currBinding.Users.Delete(o.Users...)

			_, err = o.Client.RoleBindings(o.BindingNamespace).Update(&currBinding)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
