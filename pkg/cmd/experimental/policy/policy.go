package policy

import (
	"fmt"
	"strings"

	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

func NewCommandPolicy(f *clientcmd.Factory, parentName, name string) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "manage authorization policy",
		Long:  `manage authorization policy`,
		Run:   runHelp,
	}

	cmds.AddCommand(NewCmdAddUser(f))
	cmds.AddCommand(NewCmdRemoveUser(f))
	cmds.AddCommand(NewCmdRemoveUserFromProject(f))
	cmds.AddCommand(NewCmdAddGroup(f))
	cmds.AddCommand(NewCmdRemoveGroup(f))
	cmds.AddCommand(NewCmdRemoveGroupFromProject(f))
	cmds.AddCommand(NewCmdWhoCan(f))

	return cmds
}

func runHelp(cmd *cobra.Command, args []string) {
	cmd.Help()
}

func getFlagString(cmd *cobra.Command, flag string) string {
	f := cmd.Flags().Lookup(flag)
	if f == nil {
		glog.Fatalf("Flag accessed but not defined for command %s: %s", cmd.Name(), flag)
	}
	return f.Value.String()
}

func getUniqueName(basename string, existingNames *util.StringSet) string {
	if !existingNames.Has(basename) {
		return basename
	}

	for i := 0; i < 100; i++ {
		trialName := fmt.Sprintf("%v-%d", basename, i)
		if !existingNames.Has(trialName) {
			return trialName
		}
	}

	return string(util.NewUUID())
}

func getExistingRoleBindingsForRole(roleNamespace, role string, bindingInterface client.PolicyBindingInterface) ([]*authorizationapi.RoleBinding, error) {
	existingBindings, err := bindingInterface.Get(roleNamespace)
	if err != nil && !strings.Contains(err.Error(), " not found") {
		return nil, err
	}

	ret := make([]*authorizationapi.RoleBinding, 0)
	// see if we can find an existing binding that points to the role in question.
	for _, currBinding := range existingBindings.RoleBindings {
		if currBinding.RoleRef.Name == role {
			t := currBinding
			ret = append(ret, &t)
		}
	}

	return ret, nil
}

func getExistingRoleBindingNames(bindingInterface client.PolicyBindingInterface) (*util.StringSet, error) {
	policyBindings, err := bindingInterface.List(klabels.Everything(), klabels.Everything())
	if err != nil {
		return nil, err
	}

	ret := &util.StringSet{}
	for _, existingBindings := range policyBindings.Items {
		for _, currBinding := range existingBindings.RoleBindings {
			ret.Insert(currBinding.Name)
		}
	}

	return ret, nil
}
