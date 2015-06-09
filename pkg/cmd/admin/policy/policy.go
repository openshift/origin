package policy

import (
	"fmt"
	"io"

	kapierrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const PolicyRecommendedName = "policy"

// NewCmdPolicy implements the OpenShift cli policy command
func NewCmdPolicy(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Manage authorization policy",
		Long:  `Manage authorization policy`,
		Run:   cmdutil.DefaultSubCommandRun(out),
	}

	cmds.AddCommand(NewCmdWhoCan(WhoCanRecommendedName, fullName+" "+WhoCanRecommendedName, f, out))

	cmds.AddCommand(NewCmdAddRoleToUser(AddRoleToUserRecommendedName, fullName+" "+AddRoleToUserRecommendedName, f, out))
	cmds.AddCommand(NewCmdRemoveRoleFromUser(RemoveRoleFromUserRecommendedName, fullName+" "+RemoveRoleFromUserRecommendedName, f, out))
	cmds.AddCommand(NewCmdRemoveUserFromProject(RemoveUserRecommendedName, fullName+" "+RemoveUserRecommendedName, f, out))
	cmds.AddCommand(NewCmdAddRoleToGroup(AddRoleToGroupRecommendedName, fullName+" "+AddRoleToGroupRecommendedName, f, out))
	cmds.AddCommand(NewCmdRemoveRoleFromGroup(RemoveRoleFromGroupRecommendedName, fullName+" "+RemoveRoleFromGroupRecommendedName, f, out))
	cmds.AddCommand(NewCmdRemoveGroupFromProject(RemoveGroupRecommendedName, fullName+" "+RemoveGroupRecommendedName, f, out))

	cmds.AddCommand(NewCmdAddClusterRoleToUser(AddClusterRoleToUserRecommendedName, fullName+" "+AddClusterRoleToUserRecommendedName, f, out))
	cmds.AddCommand(NewCmdRemoveClusterRoleFromUser(RemoveClusterRoleFromUserRecommendedName, fullName+" "+RemoveClusterRoleFromUserRecommendedName, f, out))
	cmds.AddCommand(NewCmdAddClusterRoleToGroup(AddClusterRoleToGroupRecommendedName, fullName+" "+AddClusterRoleToGroupRecommendedName, f, out))
	cmds.AddCommand(NewCmdRemoveClusterRoleFromGroup(RemoveClusterRoleFromGroupRecommendedName, fullName+" "+RemoveClusterRoleFromGroupRecommendedName, f, out))

	return cmds
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

// RoleBindingAccessor is used by role modification commands to access and modify roles
type RoleBindingAccessor interface {
	GetExistingRoleBindingsForRole(roleNamespace, role string) ([]*authorizationapi.RoleBinding, error)
	GetExistingRoleBindingNames() (*util.StringSet, error)
	UpdateRoleBinding(binding *authorizationapi.RoleBinding) error
	CreateRoleBinding(binding *authorizationapi.RoleBinding) error
}

// LocalRoleBindingAccessor operates against role bindings in namespace
type LocalRoleBindingAccessor struct {
	BindingNamespace string
	Client           client.Interface
}

func NewLocalRoleBindingAccessor(bindingNamespace string, client client.Interface) LocalRoleBindingAccessor {
	return LocalRoleBindingAccessor{bindingNamespace, client}
}

func (a LocalRoleBindingAccessor) GetExistingRoleBindingsForRole(roleNamespace, role string) ([]*authorizationapi.RoleBinding, error) {
	existingBindings, err := a.Client.PolicyBindings(a.BindingNamespace).Get(authorizationapi.GetPolicyBindingName(roleNamespace))
	if err != nil && !kapierrors.IsNotFound(err) {
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

func (a LocalRoleBindingAccessor) GetExistingRoleBindingNames() (*util.StringSet, error) {
	policyBindings, err := a.Client.PolicyBindings(a.BindingNamespace).List(labels.Everything(), fields.Everything())
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

func (a LocalRoleBindingAccessor) UpdateRoleBinding(binding *authorizationapi.RoleBinding) error {
	_, err := a.Client.RoleBindings(a.BindingNamespace).Update(binding)
	return err
}

func (a LocalRoleBindingAccessor) CreateRoleBinding(binding *authorizationapi.RoleBinding) error {
	binding.Namespace = a.BindingNamespace
	_, err := a.Client.RoleBindings(a.BindingNamespace).Create(binding)
	return err
}

// ClusterRoleBindingAccessor operates against cluster scoped role bindings
type ClusterRoleBindingAccessor struct {
	Client client.Interface
}

func NewClusterRoleBindingAccessor(client client.Interface) ClusterRoleBindingAccessor {
	// the master namespace value doesn't matter because we're round tripping all the values, so the namespace gets stripped out
	return ClusterRoleBindingAccessor{client}
}

func (a ClusterRoleBindingAccessor) GetExistingRoleBindingsForRole(roleNamespace, role string) ([]*authorizationapi.RoleBinding, error) {
	uncast, err := a.Client.ClusterPolicyBindings().Get(authorizationapi.GetPolicyBindingName(roleNamespace))
	if err != nil && !kapierrors.IsNotFound(err) {
		return nil, err
	}
	existingBindings := authorizationapi.ToPolicyBinding(uncast)

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

func (a ClusterRoleBindingAccessor) GetExistingRoleBindingNames() (*util.StringSet, error) {
	uncast, err := a.Client.ClusterPolicyBindings().List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, err
	}
	policyBindings := authorizationapi.ToPolicyBindingList(uncast)

	ret := &util.StringSet{}
	for _, existingBindings := range policyBindings.Items {
		for _, currBinding := range existingBindings.RoleBindings {
			ret.Insert(currBinding.Name)
		}
	}

	return ret, nil
}

func (a ClusterRoleBindingAccessor) UpdateRoleBinding(binding *authorizationapi.RoleBinding) error {
	clusterBinding := authorizationapi.ToClusterRoleBinding(binding)
	_, err := a.Client.ClusterRoleBindings().Update(clusterBinding)
	return err
}

func (a ClusterRoleBindingAccessor) CreateRoleBinding(binding *authorizationapi.RoleBinding) error {
	clusterBinding := authorizationapi.ToClusterRoleBinding(binding)
	_, err := a.Client.ClusterRoleBindings().Create(clusterBinding)
	return err
}
