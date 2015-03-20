package policy

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

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
		Use:   "remove-group <role> <group> [group]...",
		Short: "remove group from role",
		Long:  `remove group from role`,
		Run: func(cmd *cobra.Command, args []string) {
			if !options.complete(cmd) {
				return
			}

			var err error
			if options.Client, _, err = f.Clients(); err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}
			if options.BindingNamespace, err = f.DefaultNamespace(); err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}
			if err := options.Run(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleNamespace, "role-namespace", bootstrappolicy.DefaultMasterAuthorizationNamespace, "namespace where the role is located.")

	return cmd
}

func (o *RemoveGroupOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) < 2 {
		cmd.Help()
		return false
	}

	o.RoleName = args[0]
	o.Groups = args[1:]
	return true
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
