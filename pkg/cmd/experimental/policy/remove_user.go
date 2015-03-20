package policy

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

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
		Use:   "remove-user <role> <user> [user]...",
		Short: "remove user from role",
		Long:  `remove user from role`,
		Run: func(cmd *cobra.Command, args []string) {
			if !options.complete(cmd) {
				return
			}

			var err error
			if options.client, _, err = f.Clients(); err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}
			if options.bindingNamespace, err = f.DefaultNamespace(); err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}
			if err := options.run(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.roleNamespace, "role-namespace", bootstrappolicy.DefaultMasterAuthorizationNamespace, "namespace where the role is located.")

	return cmd
}

func (o *removeUserOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) < 2 {
		cmd.Help()
		return false
	}

	o.roleName = args[0]
	o.users = args[1:]
	return true
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
