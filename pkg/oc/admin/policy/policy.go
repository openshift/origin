package policy

import (
	"fmt"
	"io"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	ktemplates "k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/spf13/cobra"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationtypedclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const PolicyRecommendedName = "policy"

var policyLong = ktemplates.LongDesc(`
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

	groups := ktemplates.CommandGroups{
		{
			Message: "Discover:",
			Commands: []*cobra.Command{
				NewCmdWhoCan(WhoCanRecommendedName, fullName+" "+WhoCanRecommendedName, f, out),
				NewCmdSccSubjectReview(SubjectReviewRecommendedName, fullName+" "+SubjectReviewRecommendedName, f, out),
				NewCmdSccReview(ReviewRecommendedName, fullName+" "+ReviewRecommendedName, f, out),
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
	GetRoleBinding(name string) (*authorizationapi.RoleBinding, error)
	UpdateRoleBinding(binding *authorizationapi.RoleBinding) error
	CreateRoleBinding(binding *authorizationapi.RoleBinding) error
	DeleteRoleBinding(name string) error
}

// LocalRoleBindingAccessor operates against role bindings in namespace
type LocalRoleBindingAccessor struct {
	BindingNamespace string
	Client           authorizationtypedclient.RoleBindingsGetter
}

func NewLocalRoleBindingAccessor(bindingNamespace string, client authorizationtypedclient.RoleBindingsGetter) LocalRoleBindingAccessor {
	return LocalRoleBindingAccessor{bindingNamespace, client}
}

func (a LocalRoleBindingAccessor) GetExistingRoleBindingsForRole(roleNamespace, role string) ([]*authorizationapi.RoleBinding, error) {
	existingBindings, err := a.Client.RoleBindings(a.BindingNamespace).List(metav1.ListOptions{})
	if err != nil && !kapierrors.IsNotFound(err) {
		return nil, err
	}

	ret := make([]*authorizationapi.RoleBinding, 0)
	// see if we can find an existing binding that points to the role in question.
	for i := range existingBindings.Items {
		// shallow copy outside of the loop so that we can take its address
		currBinding := existingBindings.Items[i]
		if currBinding.RoleRef.Name == role && currBinding.RoleRef.Namespace == roleNamespace {
			ret = append(ret, &currBinding)
		}
	}

	return ret, nil
}

func (a LocalRoleBindingAccessor) GetExistingRoleBindingNames() (*sets.String, error) {
	roleBindings, err := a.Client.RoleBindings(a.BindingNamespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	ret := &sets.String{}
	for _, currBinding := range roleBindings.Items {
		ret.Insert(currBinding.Name)
	}

	return ret, nil
}

func (a LocalRoleBindingAccessor) GetRoleBinding(name string) (*authorizationapi.RoleBinding, error) {
	return a.Client.RoleBindings(a.BindingNamespace).Get(name, metav1.GetOptions{})
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

func (a LocalRoleBindingAccessor) DeleteRoleBinding(name string) error {
	return a.Client.RoleBindings(a.BindingNamespace).Delete(name, &metav1.DeleteOptions{})
}

// ClusterRoleBindingAccessor operates against cluster scoped role bindings
type ClusterRoleBindingAccessor struct {
	Client authorizationtypedclient.ClusterRoleBindingsGetter
}

func NewClusterRoleBindingAccessor(client authorizationtypedclient.ClusterRoleBindingsGetter) ClusterRoleBindingAccessor {
	// the master namespace value doesn't matter because we're round tripping all the values, so the namespace gets stripped out
	return ClusterRoleBindingAccessor{client}
}

func (a ClusterRoleBindingAccessor) GetExistingRoleBindingsForRole(roleNamespace, role string) ([]*authorizationapi.RoleBinding, error) {
	// cluster role bindings can only reference cluster roles
	if roleNamespace != "" {
		return nil, nil
	}

	existingBindings, err := a.Client.ClusterRoleBindings().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	ret := make([]*authorizationapi.RoleBinding, 0)
	// see if we can find an existing binding that points to the role in question.
	for i := range existingBindings.Items {
		// shallow copy outside of the loop so that we can take its address
		currBinding := existingBindings.Items[i]
		if currBinding.RoleRef.Name == role && currBinding.RoleRef.Namespace == "" {
			ret = append(ret, authorizationapi.ToRoleBinding(&currBinding))
		}
	}

	return ret, nil
}

func (a ClusterRoleBindingAccessor) GetExistingRoleBindingNames() (*sets.String, error) {
	existingBindings, err := a.Client.ClusterRoleBindings().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	ret := &sets.String{}
	for _, currBinding := range existingBindings.Items {
		ret.Insert(currBinding.Name)
	}

	return ret, nil
}

func (a ClusterRoleBindingAccessor) GetRoleBinding(name string) (*authorizationapi.RoleBinding, error) {
	clusterRole, err := a.Client.ClusterRoleBindings().Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	roleBinding := authorizationapi.ToRoleBinding(clusterRole)
	return roleBinding, nil
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

func (a ClusterRoleBindingAccessor) DeleteRoleBinding(name string) error {
	return a.Client.ClusterRoleBindings().Delete(name, &metav1.DeleteOptions{})
}
