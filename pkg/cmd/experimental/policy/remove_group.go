package policy

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type RemoveGroupOptions struct {
	RoleNamespace    string
	RoleName         string
	BindingNamespace string
	Client           client.Interface

	Groups []string
}

func NewCmdRemoveGroup(f *clientcmd.Factory) *cobra.Command {
	options := &RemoveGroupOptions{}

	cmd := &cobra.Command{
		Use:   "remove-role-from-group <role> <group> [group]...",
		Short: "remove group from role",
		Long:  `remove group from role`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.complete(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			var err error
			if options.Client, _, err = f.Clients(); err != nil {
				kcmdutil.CheckErr(err)
			}
			if options.BindingNamespace, err = f.DefaultNamespace(); err != nil {
				kcmdutil.CheckErr(err)
			}
			if err := options.Run(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleNamespace, "role-namespace", bootstrappolicy.DefaultMasterAuthorizationNamespace, "namespace where the role is located.")

	return cmd
}

func (o *RemoveGroupOptions) complete(args []string) error {
	if len(args) < 2 {
		return errors.New("You must specify at least two arguments: <role> <group> [group]...")
	}

	o.RoleName = args[0]
	o.Groups = args[1:]
	return nil
}

func (o *RemoveGroupOptions) Run() error {
	roleBindings, err := getExistingRoleBindingsForRole(o.RoleNamespace, o.RoleName, o.Client.PolicyBindings(o.BindingNamespace))
	if err != nil {
		return err
	}
	if len(roleBindings) == 0 {
		return fmt.Errorf("unable to locate RoleBinding for %v::%v in %v", o.RoleNamespace, o.RoleName, o.BindingNamespace)
	}

	for _, roleBinding := range roleBindings {
		roleBinding.Groups.Delete(o.Groups...)

		_, err = o.Client.RoleBindings(o.BindingNamespace).Update(roleBinding)
		if err != nil {
			return err
		}
	}

	return nil
}
