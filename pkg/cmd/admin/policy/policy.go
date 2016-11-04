package policy

import (
	"fmt"
	"io"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/uuid"

	"github.com/spf13/cobra"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const PolicyRecommendedName = "policy"

var policyLong = templates.LongDesc(`
	Manage policy on the cluster

	These commands allow you to assign and manage the roles and policies that apply to users. The reconcile
	commands allow you to reset and upgrade your system policies to the latest default policies.

	To see more information on roles and policies, use the 'get' and 'describe' commands on the following
	resources: 'clusterroles', 'clusterpolicy', 'clusterrolebindings', 'roles', 'policy', 'rolebindings',
	and 'scc'.`)

// NewCmdPolicy implements the OpenShift cli policy command
func NewCmdPolicy(name, fullName string, f *clientcmd.Factory, out, errout io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Manage policy",
		Long:  policyLong,
		Run:   cmdutil.DefaultSubCommandRun(out),
	}

	groups := templates.CommandGroups{
		{
			Message: "Discover:",
			Commands: []*cobra.Command{
				NewCmdWhoCan(WhoCanRecommendedName, fullName+" "+WhoCanRecommendedName, f, out),
			},
		},
		{
			Message: "Manage project membership:",
			Commands: []*cobra.Command{
				NewCmdRemoveUserFromProject(RemoveUserRecommendedName, fullName+" "+RemoveUserRecommendedName, f, out),
				NewCmdRemoveGroupFromProject(RemoveGroupRecommendedName, fullName+" "+RemoveGroupRecommendedName, f, out),
			},
		},
		{
			Message: "Assign roles to users and groups:",
			Commands: []*cobra.Command{
				NewCmdAddRoleToUser(AddRoleToUserRecommendedName, fullName+" "+AddRoleToUserRecommendedName, f, out),
				NewCmdAddRoleToGroup(AddRoleToGroupRecommendedName, fullName+" "+AddRoleToGroupRecommendedName, f, out),
				NewCmdRemoveRoleFromUser(RemoveRoleFromUserRecommendedName, fullName+" "+RemoveRoleFromUserRecommendedName, f, out),
				NewCmdRemoveRoleFromGroup(RemoveRoleFromGroupRecommendedName, fullName+" "+RemoveRoleFromGroupRecommendedName, f, out),
			},
		},
		{
			Message: "Assign cluster roles to users and groups:",
			Commands: []*cobra.Command{
				NewCmdAddClusterRoleToUser(AddClusterRoleToUserRecommendedName, fullName+" "+AddClusterRoleToUserRecommendedName, f, out),
				NewCmdAddClusterRoleToGroup(AddClusterRoleToGroupRecommendedName, fullName+" "+AddClusterRoleToGroupRecommendedName, f, out),
				NewCmdRemoveClusterRoleFromUser(RemoveClusterRoleFromUserRecommendedName, fullName+" "+RemoveClusterRoleFromUserRecommendedName, f, out),
				NewCmdRemoveClusterRoleFromGroup(RemoveClusterRoleFromGroupRecommendedName, fullName+" "+RemoveClusterRoleFromGroupRecommendedName, f, out),
			},
		},
		{
			Message: "Manage policy on pods and containers:",
			Commands: []*cobra.Command{
				NewCmdAddSCCToUser(AddSCCToUserRecommendedName, fullName+" "+AddSCCToUserRecommendedName, f, out),
				NewCmdAddSCCToGroup(AddSCCToGroupRecommendedName, fullName+" "+AddSCCToGroupRecommendedName, f, out),
				NewCmdRemoveSCCFromUser(RemoveSCCFromUserRecommendedName, fullName+" "+RemoveSCCFromUserRecommendedName, f, out),
				NewCmdRemoveSCCFromGroup(RemoveSCCFromGroupRecommendedName, fullName+" "+RemoveSCCFromGroupRecommendedName, f, out),
			},
		},
		{
			Message: "Upgrade and repair system policy:",
			Commands: []*cobra.Command{
				NewCmdReconcileClusterRoles(ReconcileClusterRolesRecommendedName, fullName+" "+ReconcileClusterRolesRecommendedName, f, out, errout),
				NewCmdReconcileClusterRoleBindings(ReconcileClusterRoleBindingsRecommendedName, fullName+" "+ReconcileClusterRoleBindingsRecommendedName, f, out, errout),
				NewCmdReconcileSCC(ReconcileSCCRecommendedName, fullName+" "+ReconcileSCCRecommendedName, f, out),
			},
		},
	}
	groups.Add(cmds)
	templates.ActsAsRootCommand(cmds, []string{"options"}, groups...)

	return cmds
}

func getUniqueName(basename string, existingNames *sets.String) string {
	if !existingNames.Has(basename) {
		return basename
	}

	for i := 0; i < 100; i++ {
		trialName := fmt.Sprintf("%v-%d", basename, i)
		if !existingNames.Has(trialName) {
			return trialName
		}
	}

	return string(uuid.NewUUID())
}

// RoleBindingAccessor is used by role modification commands to access and modify roles
type RoleBindingAccessor interface {
	GetExistingRoleBindingsForRole(roleNamespace, role string) ([]*authorizationapi.RoleBinding, error)
	GetExistingRoleBindingNames() (*sets.String, error)
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
			ret = append(ret, t)
		}
	}

	return ret, nil
}

func (a LocalRoleBindingAccessor) GetExistingRoleBindingNames() (*sets.String, error) {
	policyBindings, err := a.Client.PolicyBindings(a.BindingNamespace).List(kapi.ListOptions{})
	if err != nil {
		return nil, err
	}

	ret := &sets.String{}
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
			ret = append(ret, t)
		}
	}

	return ret, nil
}

func (a ClusterRoleBindingAccessor) GetExistingRoleBindingNames() (*sets.String, error) {
	uncast, err := a.Client.ClusterPolicyBindings().List(kapi.ListOptions{})
	if err != nil {
		return nil, err
	}
	policyBindings := authorizationapi.ToPolicyBindingList(uncast)

	ret := &sets.String{}
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
