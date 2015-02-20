package policy

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type removeUserOptions struct {
	roleNamespace    string
	roleName         string
	bindingNamespace string
	client           client.Interface

	userNames []string
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

	cmd.Flags().StringVar(&options.roleNamespace, "role-namespace", "master", "namespace where the role is located.")

	return cmd
}

func (o *removeUserOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) < 2 {
		cmd.Help()
		return false
	}

	o.roleName = args[0]
	o.userNames = args[1:]
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
		users := util.StringSet{}
		users.Insert(roleBinding.UserNames...)
		users.Delete(o.userNames...)
		roleBinding.UserNames = users.List()

		_, err = o.client.RoleBindings(o.bindingNamespace).Update(roleBinding)
		if err != nil {
			return err
		}
	}

	return nil
}
