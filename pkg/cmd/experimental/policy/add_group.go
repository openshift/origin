package policy

import (
	"errors"

	"github.com/spf13/cobra"

	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type addGroupOptions struct {
	roleNamespace    string
	roleName         string
	bindingNamespace string
	client           client.Interface

	groups []string
}

func NewCmdAddGroup(f *clientcmd.Factory) *cobra.Command {
	options := &addGroupOptions{}

	cmd := &cobra.Command{
		Use:   "add-role-to-group <role> <group> [group]...",
		Short: "add groups to a role",
		Long:  `add groups to a role`,
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

func (o *addGroupOptions) complete(args []string) error {
	if len(args) < 2 {
		return errors.New("You must specify at least two arguments: <role> <group> [group]...")
	}

	o.roleName = args[0]
	o.groups = args[1:]
	return nil
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

	var roleBinding *authorizationapi.RoleBinding
	isUpdate := true
	if len(roleBindings) == 0 {
		roleBinding = &authorizationapi.RoleBinding{Groups: util.NewStringSet()}
		isUpdate = false
	} else {
		// only need to add the user or group to a single roleBinding on the role.  Just choose the first one
		roleBinding = roleBindings[0]
	}

	roleBinding.RoleRef.Namespace = o.roleNamespace
	roleBinding.RoleRef.Name = o.roleName

	roleBinding.Groups.Insert(o.groups...)

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
