package policy

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	rbacv1 "k8s.io/api/rbac/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	userv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	authorizationutil "github.com/openshift/origin/pkg/authorization/util"
)

const (
	AddRoleToGroupRecommendedName      = "add-role-to-group"
	AddRoleToUserRecommendedName       = "add-role-to-user"
	RemoveRoleFromGroupRecommendedName = "remove-role-from-group"
	RemoveRoleFromUserRecommendedName  = "remove-role-from-user"

	AddClusterRoleToGroupRecommendedName      = "add-cluster-role-to-group"
	AddClusterRoleToUserRecommendedName       = "add-cluster-role-to-user"
	RemoveClusterRoleFromGroupRecommendedName = "remove-cluster-role-from-group"
	RemoveClusterRoleFromUserRecommendedName  = "remove-cluster-role-from-user"
)

var (
	addRoleToUserExample = templates.Examples(`
		# Add the 'view' role to user1 for the current project
	  %[1]s view user1

	  # Add the 'edit' role to serviceaccount1 for the current project
	  %[1]s edit -z serviceaccount1`)

	addRoleToUserLongDesc = templates.LongDesc(`
	  Add a role to users or service accounts for the current project

	  This command allows you to grant a user access to specific resources and actions within the current project, by assigning them to a role. It creates or modifies a RoleBinding referencing the specified role adding the user(s) or serviceaccount(s) to the list of subjects. The command does not require that the matching role or user/serviceaccount resources exist and will create the binding successfully even when the role or user/serviceaccount do not exist or when the user does not have access to view them.

	  If the --rolebinding-name argument is supplied, it will look for an existing rolebinding with that name. The role on the matching rolebinding MUST match the role name supplied to the command. If no rolebinding name is given, a default name will be used. When --role-namespace argument is specified as a non-empty value, it MUST match the current namespace. When role-namespace is specified, the rolebinding will reference a namespaced Role. Otherwise, the rolebinding will reference a ClusterRole resource.

	  To learn more, see information about RBAC and policy, or use the 'get' and 'describe' commands on the following resources: 'clusterroles', 'clusterrolebindings', 'roles', 'rolebindings', 'users', 'groups', and 'serviceaccounts'.`)

	addRoleToGroupLongDesc = templates.LongDesc(`
	  Add a role to groups for the current project

	  This command allows you to grant a group access to specific resources and actions within the current project, by assigning them to a role. It creates or modifies a RoleBinding referencing the specified role adding the group(s) to the list of subjects. The command does not require that the matching role or group resources exist and will create the binding successfully even when the role or group do not exist or when the user does not have access to view them.

	  If the --rolebinding-name argument is supplied, it will look for an existing rolebinding with that name. The role on the matching rolebinding MUST match the role name supplied to the command. If no rolebinding name is given, a default name will be used. When --role-namespace argument is specified as a non-empty value, it MUST match the current namespace. When role-namespace is specified, the rolebinding will reference a namespaced Role. Otherwise, the rolebinding will reference a ClusterRole resource.

	  To learn more, see information about RBAC and policy, or use the 'get' and 'describe' commands on the following resources: 'clusterroles', 'clusterrolebindings', 'roles', 'rolebindings', 'users', 'groups', and 'serviceaccounts'.`)

	addClusterRoleToUserLongDesc = templates.LongDesc(`
	  Add a role to users or service accounts across all projects

	  This command allows you to grant a user access to specific resources and actions within the cluster, by assigning them to a role. It creates or modifies a ClusterRoleBinding referencing the specified ClusterRole, adding the user(s) or serviceaccount(s) to the list of subjects. This command does not require that the matching cluster role or user/serviceaccount resources exist and will create the binding successfully even when the role or user/serviceaccount do not exist or when the user does not have access to view them.

	  If the --rolebinding-name argument is supplied, it will look for an existing clusterrolebinding with that name. The role on the matching clusterrolebinding MUST match the role name supplied to the command. If no rolebinding name is given, a default name will be used.

	  To learn more, see information about RBAC and policy, or use the 'get' and 'describe' commands on the following resources: 'clusterroles', 'clusterrolebindings', 'roles', 'rolebindings', 'users', 'groups', and 'serviceaccounts'.`)

	addClusterRoleToGroupLongDesc = templates.LongDesc(`
	  Add a role to groups for the current project

	  This command creates or modifies a ClusterRoleBinding with the named cluster role by adding the named group(s) to the list of subjects. The command does not require the matching role or group resources exist and will create the binding successfully even when the role or group do not exist or when the user does not have access to view them.

	  If the --rolebinding-name argument is supplied, it will look for an existing clusterrolebinding with that name. The role on the matching clusterrolebinding MUST match the role name supplied to the command. If no rolebinding name is given, a default name will be used.`)
)

type RoleModificationOptions struct {
	RoleName             string
	RoleNamespace        string
	RoleKind             string
	RoleBindingName      string
	RoleBindingNamespace string
	RbacClient           rbacv1client.RbacV1Interface
	SANames              []string

	UserClient           userv1client.UserV1Interface
	ServiceAccountClient corev1client.ServiceAccountsGetter

	Targets  []string
	Users    []string
	Groups   []string
	Subjects []rbacv1.Subject

	DryRun bool
	Output string

	PrintObj  func(obj runtime.Object) error
	PrintErrf func(format string, args ...interface{})

	genericclioptions.IOStreams
}

func NewRoleModificationOptions(streams genericclioptions.IOStreams) *RoleModificationOptions {
	return &RoleModificationOptions{
		IOStreams: streams,
	}
}

// NewCmdAddRoleToGroup implements the OpenShift cli add-role-to-group command
func NewCmdAddRoleToGroup(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRoleModificationOptions(streams)
	cmd := &cobra.Command{
		Use:   name + " ROLE GROUP [GROUP ...]",
		Short: "Add a role to groups for the current project",
		Long:  addRoleToGroupLongDesc,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args, &o.Groups, "group"))
			kcmdutil.CheckErr(o.checkRoleBindingNamespace(f))
			kcmdutil.CheckErr(o.AddRole())
			if len(o.Output) == 0 {
				printSuccessForCommand(o.RoleName, true, "group", o.Targets, true, o.DryRun, o.Out)
			}
		},
	}

	cmd.Flags().StringVar(&o.RoleBindingName, "rolebinding-name", o.RoleBindingName, "Name of the rolebinding to modify or create. If left empty creates a new rolebinding with a default name")
	cmd.Flags().StringVar(&o.RoleNamespace, "role-namespace", o.RoleNamespace, "namespace where the role is located: empty means a role defined in cluster policy")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

// NewCmdAddRoleToUser implements the OpenShift cli add-role-to-user command
func NewCmdAddRoleToUser(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRoleModificationOptions(streams)
	o.SANames = []string{}
	cmd := &cobra.Command{
		Use:     name + " ROLE (USER | -z SERVICEACCOUNT) [USER ...]",
		Short:   "Add a role to users or serviceaccounts for the current project",
		Long:    addRoleToUserLongDesc,
		Example: fmt.Sprintf(addRoleToUserExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.CompleteUserWithSA(f, cmd, args))
			kcmdutil.CheckErr(o.checkRoleBindingNamespace(f))
			kcmdutil.CheckErr(o.AddRole())
			if len(o.Output) == 0 {
				printSuccessForCommand(o.RoleName, true, "user", o.Targets, true, o.DryRun, o.Out)
			}
		},
	}

	cmd.Flags().StringVar(&o.RoleBindingName, "rolebinding-name", o.RoleBindingName, "Name of the rolebinding to modify or create. If left empty creates a new rolebinding with a default name")
	cmd.Flags().StringVar(&o.RoleNamespace, "role-namespace", o.RoleNamespace, "namespace where the role is located: empty means a role defined in cluster policy")
	cmd.Flags().StringSliceVarP(&o.SANames, "serviceaccount", "z", o.SANames, "service account in the current namespace to use as a user")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

// NewCmdRemoveRoleFromGroup implements the OpenShift cli remove-role-from-group command
func NewCmdRemoveRoleFromGroup(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRoleModificationOptions(streams)
	cmd := &cobra.Command{
		Use:   name + " ROLE GROUP [GROUP ...]",
		Short: "Remove a role from groups for the current project",
		Long:  `Remove a role from groups for the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args, &o.Groups, "group"))
			kcmdutil.CheckErr(o.checkRoleBindingNamespace(f))
			kcmdutil.CheckErr(o.RemoveRole())
			if len(o.Output) == 0 {
				printSuccessForCommand(o.RoleName, false, "group", o.Targets, true, o.DryRun, o.Out)
			}
		},
	}

	cmd.Flags().StringVar(&o.RoleBindingName, "rolebinding-name", o.RoleBindingName, "Name of the rolebinding to modify. If left empty it will operate on all rolebindings")
	cmd.Flags().StringVar(&o.RoleNamespace, "role-namespace", o.RoleNamespace, "namespace where the role is located: empty means a role defined in cluster policy")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

// NewCmdRemoveRoleFromUser implements the OpenShift cli remove-role-from-user command
func NewCmdRemoveRoleFromUser(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRoleModificationOptions(streams)
	o.SANames = []string{}
	cmd := &cobra.Command{
		Use:   name + " ROLE USER [USER ...]",
		Short: "Remove a role from users for the current project",
		Long:  `Remove a role from users for the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.CompleteUserWithSA(f, cmd, args))
			kcmdutil.CheckErr(o.checkRoleBindingNamespace(f))
			kcmdutil.CheckErr(o.RemoveRole())
			if len(o.Output) == 0 {
				printSuccessForCommand(o.RoleName, false, "user", o.Targets, true, o.DryRun, o.Out)
			}
		},
	}

	cmd.Flags().StringVar(&o.RoleBindingName, "rolebinding-name", o.RoleBindingName, "Name of the rolebinding to modify. If left empty it will operate on all rolebindings")
	cmd.Flags().StringVar(&o.RoleNamespace, "role-namespace", o.RoleNamespace, "namespace where the role is located: empty means a role defined in cluster policy")
	cmd.Flags().StringSliceVarP(&o.SANames, "serviceaccount", "z", o.SANames, "service account in the current namespace to use as a user")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

// NewCmdAddClusterRoleToGroup implements the OpenShift cli add-cluster-role-to-group command
func NewCmdAddClusterRoleToGroup(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRoleModificationOptions(streams)
	o.RoleKind = "ClusterRole"
	cmd := &cobra.Command{
		Use:   name + " <role> <group> [group]...",
		Short: "Add a role to groups for all projects in the cluster",
		Long:  addClusterRoleToGroupLongDesc,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args, &o.Groups, "group"))
			kcmdutil.CheckErr(o.AddRole())
			if len(o.Output) == 0 {
				printSuccessForCommand(o.RoleName, true, "group", o.Targets, false, o.DryRun, o.Out)
			}
		},
	}

	cmd.Flags().StringVar(&o.RoleBindingName, "rolebinding-name", o.RoleBindingName, "Name of the rolebinding to modify or create. If left empty creates a new rolebinding with a default name")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

// NewCmdAddClusterRoleToUser implements the OpenShift cli add-cluster-role-to-user command
func NewCmdAddClusterRoleToUser(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRoleModificationOptions(streams)
	o.RoleKind = "ClusterRole"
	o.SANames = []string{}
	cmd := &cobra.Command{
		Use:   name + " <role> <user | -z serviceaccount> [user]...",
		Short: "Add a role to users for all projects in the cluster",
		Long:  addClusterRoleToUserLongDesc,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.CompleteUserWithSA(f, cmd, args))
			kcmdutil.CheckErr(o.AddRole())
			if len(o.Output) == 0 {
				printSuccessForCommand(o.RoleName, true, "user", o.Targets, false, o.DryRun, o.Out)
			}
		},
	}

	cmd.Flags().StringVar(&o.RoleBindingName, "rolebinding-name", o.RoleBindingName, "Name of the rolebinding to modify or create. If left empty creates a new rolebindo.RoleBindingNameg with a default name")
	cmd.Flags().StringSliceVarP(&o.SANames, "serviceaccount", "z", o.SANames, "service account in the current namespace to use o.SANamess a user")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

// NewCmdRemoveClusterRoleFromGroup implements the OpenShift cli remove-cluster-role-from-group command
func NewCmdRemoveClusterRoleFromGroup(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRoleModificationOptions(streams)
	o.RoleKind = "ClusterRole"
	cmd := &cobra.Command{
		Use:   name + " <role> <group> [group]...",
		Short: "Remove a role from groups for all projects in the cluster",
		Long:  `Remove a role from groups for all projects in the cluster`,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args, &o.Groups, "group"))
			kcmdutil.CheckErr(o.RemoveRole())
			if len(o.Output) == 0 {
				printSuccessForCommand(o.RoleName, false, "group", o.Targets, false, o.DryRun, o.Out)
			}
		},
	}

	cmd.Flags().StringVar(&o.RoleBindingName, "rolebinding-name", o.RoleBindingName, "Name of the rolebinding to modify. If left empty it will operate on all rolebindings")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

// NewCmdRemoveClusterRoleFromUser implements the OpenShift cli remove-cluster-role-from-user command
func NewCmdRemoveClusterRoleFromUser(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRoleModificationOptions(streams)
	o.RoleKind = "ClusterRole"
	o.SANames = []string{}
	cmd := &cobra.Command{
		Use:   name + " <role> <user> [user]...",
		Short: "Remove a role from users for all projects in the cluster",
		Long:  `Remove a role from users for all projects in the cluster`,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.CompleteUserWithSA(f, cmd, args))
			kcmdutil.CheckErr(o.RemoveRole())
			if len(o.Output) == 0 {
				printSuccessForCommand(o.RoleName, false, "user", o.Targets, false, o.DryRun, o.Out)
			}
		},
	}

	cmd.Flags().StringVar(&o.RoleBindingName, "rolebinding-name", o.RoleBindingName, "Name of the rolebinding to modify. If left empty it will operate on all rolebindings")
	cmd.Flags().StringSliceVarP(&o.SANames, "serviceaccount", "z", o.SANames, "service account in the current namespace to use as a user")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

func (o *RoleModificationOptions) checkRoleBindingNamespace(f kcmdutil.Factory) error {
	var err error
	o.RoleBindingNamespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	if len(o.RoleNamespace) > 0 {
		if o.RoleBindingNamespace != o.RoleNamespace {
			return fmt.Errorf("role binding in namespace %q can't reference role in different namespace %q",
				o.RoleBindingNamespace, o.RoleNamespace)
		}
		o.RoleKind = "Role"
	} else {
		o.RoleKind = "ClusterRole"
	}
	return nil
}

func (o *RoleModificationOptions) innerComplete(f kcmdutil.Factory, cmd *cobra.Command) error {
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.RbacClient, err = rbacv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	o.UserClient, err = userv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	o.ServiceAccountClient, err = corev1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.DryRun = kcmdutil.GetFlagBool(cmd, "dry-run")
	o.Output = kcmdutil.GetFlagString(cmd, "output")
	o.PrintObj = func(obj runtime.Object) error {
		return kcmdutil.PrintObject(cmd, obj, o.Out)
	}
	o.PrintErrf = func(format string, args ...interface{}) {
		fmt.Fprintf(o.ErrOut, format, args...)
	}

	return nil
}

func (o *RoleModificationOptions) CompleteUserWithSA(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("you must specify a role")
	}

	o.RoleName = args[0]
	if len(args) > 1 {
		o.Users = append(o.Users, args[1:]...)
	}

	o.Targets = o.Users

	if (len(o.Users) == 0) && (len(o.SANames) == 0) {
		return errors.New("you must specify at least one user or service account")
	}

	// return an error if a fully-qualified service-account name is used
	for _, sa := range o.SANames {
		if strings.HasPrefix(sa, "system:serviceaccount") {
			return errors.New("--serviceaccount (-z) should only be used with short-form serviceaccount names (e.g. `default`)")
		}

		if errCauses := validation.ValidateServiceAccountName(sa, false); len(errCauses) > 0 {
			message := fmt.Sprintf("%q is not a valid serviceaccount name:\n  ", sa)
			message += strings.Join(errCauses, "\n  ")
			return errors.New(message)
		}
	}

	err := o.innerComplete(f, cmd)
	if err != nil {
		return err
	}

	defaultNamespace, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	for _, sa := range o.SANames {
		o.Targets = append(o.Targets, sa)
		o.Subjects = append(o.Subjects, rbacv1.Subject{Namespace: defaultNamespace, Name: sa, Kind: rbacv1.ServiceAccountKind})
	}

	return nil
}

func (o *RoleModificationOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string, target *[]string, targetName string) error {
	if len(args) < 2 {
		return fmt.Errorf("you must specify at least two arguments: <role> <%s> [%s]...", targetName, targetName)
	}

	o.RoleName = args[0]
	*target = append(*target, args[1:]...)

	o.Targets = *target

	return o.innerComplete(f, cmd)
}

func (o *RoleModificationOptions) getRoleBinding() (*roleBindingAbstraction, bool /* isUpdate */, error) {
	roleBinding, err := getRoleBindingAbstraction(o.RbacClient, o.RoleBindingName, o.RoleBindingNamespace)
	if err != nil {
		if kapierrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	// Check that we update the rolebinding for the intended role.
	if roleBinding.RoleName() != o.RoleName {
		return nil, false, fmt.Errorf("rolebinding %s found for role %s, not %s",
			o.RoleBindingName, roleBinding.RoleName(), o.RoleName)
	}
	if roleBinding.RoleKind() != o.RoleKind {
		return nil, false, fmt.Errorf("rolebinding %s found for %q, not %q",
			o.RoleBindingName, roleBinding.RoleKind(), o.RoleKind)
	}

	return roleBinding, true, nil
}

func (o *RoleModificationOptions) newRoleBinding() (*roleBindingAbstraction, error) {
	var roleBindingName string

	// Create a new rolebinding with the desired name.
	if len(o.RoleBindingName) > 0 {
		roleBindingName = o.RoleBindingName
	} else {
		// If unspecified will always use the default naming
		var err error
		roleBindingName, err = getUniqueName(o.RbacClient, o.RoleName, o.RoleBindingNamespace)
		if err != nil {
			return nil, err
		}
	}
	roleBinding, err := newRoleBindingAbstraction(o.RbacClient, roleBindingName, o.RoleBindingNamespace, o.RoleName, o.RoleKind)
	if err != nil {
		return nil, err
	}
	return roleBinding, nil
}

func (o *RoleModificationOptions) AddRole() error {
	var (
		roleBinding *roleBindingAbstraction
		isUpdate    bool
		err         error
	)

	// Look for an existing rolebinding by name.
	if len(o.RoleBindingName) > 0 {
		roleBinding, isUpdate, err = o.getRoleBinding()
		if err != nil {
			return err
		}
	} else {
		// Check if we already have a role binding that matches
		checkBindings, err := getRoleBindingAbstractionsForRole(o.RbacClient, o.RoleName, o.RoleKind, o.RoleBindingNamespace)
		if err != nil {
			return err
		}
		if len(checkBindings) > 0 {
			for _, checkBinding := range checkBindings {
				newSubjects := addSubjects(o.Users, o.Groups, o.Subjects, checkBinding.Subjects())
				if len(newSubjects) == len(checkBinding.Subjects()) {
					// we already have a rolebinding that matches
					if len(o.Output) > 0 {
						return o.PrintObj(checkBinding.Object())
					}
					return nil
				}
			}
		}
	}

	if roleBinding == nil {
		roleBinding, err = o.newRoleBinding()
		if err != nil {
			return err
		}
	}

	// warn if binding to non-existent role
	if o.PrintErrf != nil {
		var err error
		if roleBinding.RoleKind() == "Role" {
			_, err = o.RbacClient.Roles(o.RoleBindingNamespace).Get(roleBinding.RoleName(), metav1.GetOptions{})
		} else {
			_, err = o.RbacClient.ClusterRoles().Get(roleBinding.RoleName(), metav1.GetOptions{})
		}
		if err != nil && kapierrors.IsNotFound(err) {
			o.PrintErrf("Warning: role '%s' not found\n", roleBinding.RoleName())
		}
	}
	existingSubjects := roleBinding.Subjects()
	newSubjects := addSubjects(o.Users, o.Groups, o.Subjects, existingSubjects)
	// warn if any new subject does not exist, skipping existing subjects on the binding
	if o.PrintErrf != nil {
		// `addSubjects` appends new subjects onto the list of existing ones, skip over the existing ones
		for _, newSubject := range newSubjects[len(existingSubjects):] {
			var err error
			switch newSubject.Kind {
			case rbacv1.ServiceAccountKind:
				if o.ServiceAccountClient != nil {
					_, err = o.ServiceAccountClient.ServiceAccounts(newSubject.Namespace).Get(newSubject.Name, metav1.GetOptions{})
				}
			case rbacv1.UserKind:
				if o.UserClient != nil {
					_, err = o.UserClient.Users().Get(newSubject.Name, metav1.GetOptions{})
				}
			case rbacv1.GroupKind:
				if o.UserClient != nil {
					_, err = o.UserClient.Groups().Get(newSubject.Name, metav1.GetOptions{})
				}
			}
			if err != nil && kapierrors.IsNotFound(err) {
				o.PrintErrf("Warning: %s '%s' not found\n", newSubject.Kind, newSubject.Name)
			}
		}
	}
	roleBinding.SetSubjects(newSubjects)

	if len(o.Output) > 0 {
		return o.PrintObj(roleBinding.Object())
	}

	if o.DryRun {
		return nil
	}

	if isUpdate {
		err = roleBinding.Update()
	} else {
		err = roleBinding.Create()
		// If the rolebinding was created in the meantime, rerun
		if kapierrors.IsAlreadyExists(err) {
			return o.AddRole()
		}
	}
	if err != nil {
		return err
	}

	return nil
}

// addSubjects appends new subjects to the list existing ones, removing any duplicates.
// !!! The returned list MUST start with `existingSubjects` and only append new subjects *after*;
//     consumers of this function expect new subjects to start at `len(existingSubjects)`.
func addSubjects(users []string, groups []string, subjects []rbacv1.Subject, existingSubjects []rbacv1.Subject) []rbacv1.Subject {
	subjectsToAdd := authorizationutil.BuildRBACSubjects(users, groups)
	subjectsToAdd = append(subjectsToAdd, subjects...)
	newSubjects := make([]rbacv1.Subject, len(existingSubjects))
	copy(newSubjects, existingSubjects)

subjectCheck:
	for _, subjectToAdd := range subjectsToAdd {
		for _, newSubject := range newSubjects {
			if newSubject.Kind == subjectToAdd.Kind &&
				newSubject.Name == subjectToAdd.Name &&
				newSubject.Namespace == subjectToAdd.Namespace {
				continue subjectCheck
			}
		}

		newSubjects = append(newSubjects, subjectToAdd)
	}

	return newSubjects
}

func (o *RoleModificationOptions) checkRolebindingAutoupdate(roleBinding *roleBindingAbstraction) {
	if roleBinding.Annotation(rbacv1.AutoUpdateAnnotationKey) == "true" {
		if o.PrintErrf != nil {
			o.PrintErrf("Warning: Your changes may get lost whenever a master"+
				" is restarted, unless you prevent reconciliation of this"+
				" rolebinding using the following command: oc annotate"+
				" %s.rbac %s '%s=false' --overwrite", roleBinding.Type(),
				roleBinding.Name(), rbacv1.AutoUpdateAnnotationKey)
		}
	}
}

func (o *RoleModificationOptions) RemoveRole() error {
	var roleBindings []*roleBindingAbstraction
	var err error
	if len(o.RoleBindingName) > 0 {
		existingRoleBinding, err := getRoleBindingAbstraction(o.RbacClient, o.RoleBindingName, o.RoleBindingNamespace)
		if err != nil {
			return err
		}
		// Check that we update the rolebinding for the intended role.
		if existingRoleBinding.RoleName() != o.RoleName {
			return fmt.Errorf("rolebinding %s contains role %s, instead of role %s",
				o.RoleBindingName, existingRoleBinding.RoleName(), o.RoleName)
		}
		if existingRoleBinding.RoleKind() != o.RoleKind {
			return fmt.Errorf("rolebinding %s contains role %s of kind %q, not %q",
				o.RoleBindingName, o.RoleName, existingRoleBinding.RoleKind(), o.RoleKind)
		}

		roleBindings = make([]*roleBindingAbstraction, 1)
		roleBindings[0] = existingRoleBinding
	} else {
		roleBindings, err = getRoleBindingAbstractionsForRole(o.RbacClient, o.RoleName, o.RoleKind, o.RoleBindingNamespace)
		if err != nil {
			return err
		}
	}
	if len(roleBindings) == 0 {
		return fmt.Errorf("unable to locate RoleBinding %s for %s %q", o.RoleBindingName, o.RoleKind, o.RoleName)
	}

	subjectsToRemove := authorizationutil.BuildRBACSubjects(o.Users, o.Groups)
	subjectsToRemove = append(subjectsToRemove, o.Subjects...)

	found := 0
	cnt := 0
	for _, roleBinding := range roleBindings {
		var resultingSubjects []rbacv1.Subject
		resultingSubjects, cnt = removeSubjects(roleBinding.Subjects(), subjectsToRemove)
		roleBinding.SetSubjects(resultingSubjects)
		found += cnt
	}

	if len(o.Output) > 0 {
		if found == 0 {
			return fmt.Errorf("unable to find target %v", o.Targets)
		}
		var updated runtime.Object
		if len(o.RoleBindingNamespace) > 0 {
			updatedBindings := &rbacv1.RoleBindingList{
				TypeMeta: metav1.TypeMeta{
					Kind:       "List",
					APIVersion: "v1",
				},
				ListMeta: metav1.ListMeta{},
			}
			for _, binding := range roleBindings {
				updatedBindings.Items = append(updatedBindings.Items, *(binding.Object().(*rbacv1.RoleBinding)))
			}
			updated = updatedBindings
		} else {
			updatedBindings := &rbacv1.ClusterRoleBindingList{
				TypeMeta: metav1.TypeMeta{
					Kind:       "List",
					APIVersion: "v1",
				},
				ListMeta: metav1.ListMeta{},
			}
			for _, binding := range roleBindings {
				updatedBindings.Items = append(updatedBindings.Items, *(binding.Object().(*rbacv1.ClusterRoleBinding)))
			}
			updated = updatedBindings
		}

		return o.PrintObj(updated)
	}

	if o.DryRun {
		return nil
	}

	for _, roleBinding := range roleBindings {
		if len(roleBinding.Subjects()) > 0 || roleBinding.Annotation(rbacv1.AutoUpdateAnnotationKey) == "false" {
			err = roleBinding.Update()
		} else {
			err = roleBinding.Delete()
		}
		if err != nil {
			return err
		}
		o.checkRolebindingAutoupdate(roleBinding)
	}
	if found == 0 {
		return fmt.Errorf("unable to find target %v", o.Targets)
	}

	return nil
}

func removeSubjects(haystack, needles []rbacv1.Subject) ([]rbacv1.Subject, int) {
	newSubjects := []rbacv1.Subject{}
	found := 0

existingLoop:
	for _, existingSubject := range haystack {
		for _, toRemove := range needles {
			if existingSubject.Kind == toRemove.Kind &&
				existingSubject.Name == toRemove.Name &&
				existingSubject.Namespace == toRemove.Namespace {
				found++
				continue existingLoop

			}
		}

		newSubjects = append(newSubjects, existingSubject)
	}

	return newSubjects, found
}

// prints affirmative output for role modification commands
func printSuccessForCommand(role string, didAdd bool, targetName string, targets []string, isNamespaced bool, dryRun bool, out io.Writer) {
	verb := "removed"
	clusterScope := "cluster "
	allTargets := fmt.Sprintf("%q", targets)
	if isNamespaced {
		clusterScope = ""
	}
	if len(targets) == 1 {
		allTargets = fmt.Sprintf("%q", targets[0])
	}
	if didAdd {
		verb = "added"
	}

	msg := "%srole %q %s: %s"
	if dryRun {
		msg += " (dry run)"
	}

	fmt.Fprintf(out, msg+"\n", clusterScope, role, verb, allTargets)
}
