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

type removeUserOptions struct {
	roleNamespace    string
	roleName         string
	bindingNamespace string
	client           client.Interface

	users []string
}

func NewCmdRemoveUser(f *clientcmd.Factory) *cobra.Command {
	options := &removeUserOptions{}

	cmd := &cobra.Command{
		Use:   "remove-role-from-user <role> <user> [user]...",
		Short: "remove user from role",
		Long:  `remove user from role`,
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

	cmd.Flags().StringVar(&options.roleNamespace, "role-namespace", bootstrappolicy.DefaultMasterAuthorizationNamespace, "namespace where the role is located.")

	return cmd
}

func (o *removeUserOptions) complete(args []string) error {
	if len(args) < 2 {
		return errors.New("You must specify at least two arguments: <role> <user> [user]...")
	}

	o.roleName = args[0]
	o.users = args[1:]
	return nil
}

func (o *removeUserOptions) run() error {
	roleBindings, err := getExistingRoleBindingsForRole(o.roleNamespace, o.roleName, o.client.PolicyBindings(o.bindingNamespace))
	if err != nil {
		return err
	}
	if len(roleBindings) == 0 {
		return fmt.Errorf("unable to locate RoleBinding for %v::%v in %v", o.roleNamespace, o.roleName, o.bindingNamespace)
	}

	for _, roleBinding := range roleBindings {
		roleBinding.Users.Delete(o.users...)

		_, err = o.client.RoleBindings(o.bindingNamespace).Update(roleBinding)
		if err != nil {
			return err
		}
	}

	return nil
}
