package policy

import (
	"fmt"
	"io"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbacclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/rbac/internalversion"
	ktemplates "k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/spf13/cobra"

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

func getUniqueName(rbacClient rbacclient.RbacInterface, basename string, namespace string) (string, error) {
	existingNames := sets.String{}

	if len(namespace) > 0 {
		roleBindings, err := rbacClient.RoleBindings(namespace).List(metav1.ListOptions{})
		if err != nil && !kapierrors.IsNotFound(err) {
			return "", err
		}
		for _, currBinding := range roleBindings.Items {
			existingNames.Insert(currBinding.Name)
		}
	} else {
		roleBindings, err := rbacClient.ClusterRoleBindings().List(metav1.ListOptions{})
		if err != nil && !kapierrors.IsNotFound(err) {
			return "", err
		}
		for _, currBinding := range roleBindings.Items {
			existingNames.Insert(currBinding.Name)
		}
	}

	if !existingNames.Has(basename) {
		return basename, nil
	}

	for i := 0; i < 100; i++ {
		trialName := fmt.Sprintf("%v-%d", basename, i)
		if !existingNames.Has(trialName) {
			return trialName, nil
		}
	}

	return string(uuid.NewUUID()), nil
}

type roleBindingAbstraction struct {
	rbacClient         rbacclient.RbacInterface
	roleBinding        *rbac.RoleBinding
	clusterRoleBinding *rbac.ClusterRoleBinding
}

func newRoleBindingAbstraction(rbacClient rbacclient.RbacInterface, name string, namespace string, roleName string, roleKind string) (*roleBindingAbstraction, error) {
	r := roleBindingAbstraction{rbacClient: rbacClient}
	if len(namespace) > 0 {
		switch roleKind {
		case "Role":
			r.roleBinding = &(rbac.NewRoleBinding(roleName, namespace).RoleBinding)
		case "ClusterRole":
			r.roleBinding = &(rbac.NewRoleBindingForClusterRole(roleName, namespace).RoleBinding)
		default:
			return nil, fmt.Errorf("Unknown Role Kind: %q", roleKind)
		}
		if name != roleName {
			r.roleBinding.Name = name
		}
	} else {
		if roleKind != "ClusterRole" {
			return nil, fmt.Errorf("Cluster Role Bindings can only reference Cluster Roles")
		}
		r.clusterRoleBinding = &(rbac.NewClusterBinding(roleName).ClusterRoleBinding)
		if name != roleName {
			r.clusterRoleBinding.Name = name
		}
	}
	return &r, nil
}

func getRoleBindingAbstraction(rbacClient rbacclient.RbacInterface, name string, namespace string) (*roleBindingAbstraction, error) {
	var err error
	r := roleBindingAbstraction{rbacClient: rbacClient}
	if len(namespace) > 0 {
		r.roleBinding, err = rbacClient.RoleBindings(namespace).Get(name, metav1.GetOptions{})
	} else {
		r.clusterRoleBinding, err = rbacClient.ClusterRoleBindings().Get(name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func getRoleBindingAbstractionsForRole(rbacClient rbacclient.RbacInterface, roleName string, roleKind string, namespace string) ([]*roleBindingAbstraction, error) {
	ret := make([]*roleBindingAbstraction, 0)
	// see if we can find an existing binding that points to the role in question.
	if len(namespace) > 0 {
		roleBindings, err := rbacClient.RoleBindings(namespace).List(metav1.ListOptions{})
		if err != nil && !kapierrors.IsNotFound(err) {
			return nil, err
		}
		for i := range roleBindings.Items {
			// shallow copy outside of the loop so that we can take its address
			roleBinding := roleBindings.Items[i]
			if roleBinding.RoleRef.Name == roleName && roleBinding.RoleRef.Kind == roleKind {
				ret = append(ret, &roleBindingAbstraction{rbacClient: rbacClient, roleBinding: &roleBinding})
			}
		}
	} else {
		clusterRoleBindings, err := rbacClient.ClusterRoleBindings().List(metav1.ListOptions{})
		if err != nil && !kapierrors.IsNotFound(err) {
			return nil, err
		}
		for i := range clusterRoleBindings.Items {
			// shallow copy outside of the loop so that we can take its address
			clusterRoleBinding := clusterRoleBindings.Items[i]
			if clusterRoleBinding.RoleRef.Name == roleName {
				ret = append(ret, &roleBindingAbstraction{rbacClient: rbacClient, clusterRoleBinding: &clusterRoleBinding})
			}
		}
	}

	return ret, nil
}

func (r roleBindingAbstraction) Name() string {
	if r.roleBinding != nil {
		return r.roleBinding.Name
	} else {
		return r.clusterRoleBinding.Name
	}
}

func (r roleBindingAbstraction) RoleName() string {
	if r.roleBinding != nil {
		return r.roleBinding.RoleRef.Name
	} else {
		return r.clusterRoleBinding.RoleRef.Name
	}
}

func (r roleBindingAbstraction) RoleKind() string {
	if r.roleBinding != nil {
		return r.roleBinding.RoleRef.Kind
	} else {
		return r.clusterRoleBinding.RoleRef.Kind
	}
}

func (r roleBindingAbstraction) Annotation(key string) string {
	if r.roleBinding != nil {
		return r.roleBinding.Annotations[key]
	} else {
		return r.clusterRoleBinding.Annotations[key]
	}
}

func (r roleBindingAbstraction) Subjects() []rbac.Subject {
	if r.roleBinding != nil {
		return r.roleBinding.Subjects
	} else {
		return r.clusterRoleBinding.Subjects
	}
}

func (r roleBindingAbstraction) SetSubjects(subjects []rbac.Subject) {
	if r.roleBinding != nil {
		r.roleBinding.Subjects = subjects
	} else {
		r.clusterRoleBinding.Subjects = subjects
	}
}

func (r roleBindingAbstraction) Object() runtime.Object {
	if r.roleBinding != nil {
		return r.roleBinding
	} else {
		return r.clusterRoleBinding
	}
}

func (r roleBindingAbstraction) Create() error {
	var err error
	if r.roleBinding != nil {
		_, err = r.rbacClient.RoleBindings(r.roleBinding.Namespace).Create(r.roleBinding)
	} else {
		_, err = r.rbacClient.ClusterRoleBindings().Create(r.clusterRoleBinding)
	}
	return err
}

func (r roleBindingAbstraction) Update() error {
	var err error
	if r.roleBinding != nil {
		_, err = r.rbacClient.RoleBindings(r.roleBinding.Namespace).Update(r.roleBinding)
	} else {
		_, err = r.rbacClient.ClusterRoleBindings().Update(r.clusterRoleBinding)
	}
	return err
}

func (r roleBindingAbstraction) Delete() error {
	var err error
	if r.roleBinding != nil {
		err = r.rbacClient.RoleBindings(r.roleBinding.Namespace).Delete(r.roleBinding.Name, &metav1.DeleteOptions{})
	} else {
		err = r.rbacClient.ClusterRoleBindings().Delete(r.clusterRoleBinding.Name, &metav1.DeleteOptions{})
	}
	return err
}
