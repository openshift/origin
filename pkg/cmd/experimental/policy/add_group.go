package policy

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type addGroupOptions struct {
	roleNamespace    string
	roleName         string
	bindingNamespace string
	client           client.Interface

	groupNames []string
}

func NewCmdAddGroup(f *clientcmd.Factory) *cobra.Command {
	options := &addGroupOptions{}

	cmd := &cobra.Command{
		Use:   "add-group <role> <group> [group]...",
		Short: "add group to role",
		Long:  `add group to role`,
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

func (o *addGroupOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) < 2 {
		cmd.Help()
		return false
	}

	o.roleName = args[0]
	o.groupNames = args[1:]
	return true
}

func (o *addGroupOptions) run() error {
	roleBindings, err := getExistingRoleBindingsForRole(o.roleNamespace, o.roleName, o.client.PolicyBindings(o.bindingNamespace))
	if err != nil {
		return err
	}
	roleBindingNames, err := getExistingRoleBindingNames(o.client.PolicyBindings(o.bindingNamespace))
	if err != nil {
		return err
	}

	roleBinding := (*authorizationapi.RoleBinding)(nil)
	isUpdate := true
	if len(roleBindings) == 0 {
		roleBinding = &authorizationapi.RoleBinding{}
		isUpdate = false
	} else {
		// only need to add the user or group to a single roleBinding on the role.  Just choose the first one
		roleBinding = roleBindings[0]
	}

	roleBinding.RoleRef.Namespace = o.roleNamespace
	roleBinding.RoleRef.Name = o.roleName

	groups := util.StringSet{}
	groups.Insert(roleBinding.GroupNames...)
	groups.Insert(o.groupNames...)
	roleBinding.GroupNames = groups.List()

	if isUpdate {
		_, err = o.client.RoleBindings(o.bindingNamespace).Update(roleBinding)
	} else {
		roleBinding.Name = getUniqueName(o.roleName, roleBindingNames)
		_, err = o.client.RoleBindings(o.bindingNamespace).Create(roleBinding)
	}
	if err != nil {
		return err
	}

	return nil
}
