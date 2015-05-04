package policy

import (
	"fmt"
	"io"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const PolicyRecommendedName = "policy"

func NewCommandPolicy(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Manage authorization policy",
		Long:  `Manage authorization policy`,
		Run:   runHelp,
	}

	cmds.AddCommand(NewCmdAddRoleToUser(AddRoleToUserRecommendedName, fullName+" "+AddRoleToUserRecommendedName, f, out))
	cmds.AddCommand(NewCmdRemoveRoleFromUser(RemoveRoleFromUserRecommendedName, fullName+" "+RemoveRoleFromUserRecommendedName, f, out))
	cmds.AddCommand(NewCmdRemoveUserFromProject(RemoveUserRecommendedName, fullName+" "+RemoveUserRecommendedName, f, out))
	cmds.AddCommand(NewCmdAddRoleToGroup(AddRoleToGroupRecommendedName, fullName+" "+AddRoleToGroupRecommendedName, f, out))
	cmds.AddCommand(NewCmdRemoveRoleFromGroup(RemoveRoleFromGroupRecommendedName, fullName+" "+RemoveRoleFromGroupRecommendedName, f, out))
	cmds.AddCommand(NewCmdRemoveGroupFromProject(RemoveGroupRecommendedName, fullName+" "+RemoveGroupRecommendedName, f, out))
	cmds.AddCommand(NewCmdWhoCan(WhoCanRecommendedName, fullName+" "+WhoCanRecommendedName, f, out))

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
	existingBindings, err := bindingInterface.Get(authorizationapi.GetPolicyBindingName(roleNamespace))
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
	policyBindings, err := bindingInterface.List(labels.Everything(), fields.Everything())
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
