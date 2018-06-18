package policy

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbacclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/rbac/internalversion"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	authorizationutil "github.com/openshift/origin/pkg/authorization/util"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
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
)

type RoleModificationOptions struct {
	RoleName             string
	RoleKind             string
	RoleBindingName      string
	RoleBindingNamespace string
	RbacClient           rbacclient.RbacInterface

	Targets  []string
	Users    []string
	Groups   []string
	Subjects []rbac.Subject

	DryRun bool
	Output string

	PrintObj func(obj runtime.Object) error
}

// NewCmdAddRoleToGroup implements the OpenShift cli add-role-to-group command
func NewCmdAddRoleToGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}
	roleNamespace := ""

	cmd := &cobra.Command{
		Use:   name + " ROLE GROUP [GROUP ...]",
		Short: "Add a role to groups for the current project",
		Long:  `Add a role to groups for the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, cmd, args, &options.Groups, "group", out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.checkRoleBindingNamespace(f, roleNamespace); err != nil {
				kcmdutil.CheckErr(err)
			}

			if err := options.AddRole(); err != nil {
				kcmdutil.CheckErr(err)
				return
			}

			if len(options.Output) == 0 {
				printSuccessForCommand(options.RoleName, true, "group", options.Targets, true, options.DryRun, out)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleBindingName, "rolebinding-name", "", "Name of the rolebinding to modify or create. If left empty creates a new rolebinding with a default name")
	cmd.Flags().StringVar(&roleNamespace, "role-namespace", "", "namespace where the role is located: empty means a role defined in cluster policy")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

// NewCmdAddRoleToUser implements the OpenShift cli add-role-to-user command
func NewCmdAddRoleToUser(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}
	saNames := []string{}
	roleNamespace := ""

	cmd := &cobra.Command{
		Use:     name + " ROLE (USER | -z SERVICEACCOUNT) [USER ...]",
		Short:   "Add a role to users or serviceaccounts for the current project",
		Long:    `Add a role to users or serviceaccounts for the current project`,
		Example: fmt.Sprintf(addRoleToUserExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.CompleteUserWithSA(f, cmd, args, saNames, out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.checkRoleBindingNamespace(f, roleNamespace); err != nil {
				kcmdutil.CheckErr(err)
			}

			if err := options.AddRole(); err != nil {
				kcmdutil.CheckErr(err)
				return
			}
			if len(options.Output) == 0 {
				printSuccessForCommand(options.RoleName, true, "user", options.Targets, true, options.DryRun, out)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleBindingName, "rolebinding-name", "", "Name of the rolebinding to modify or create. If left empty creates a new rolebinding with a default name")
	cmd.Flags().StringVar(&roleNamespace, "role-namespace", "", "namespace where the role is located: empty means a role defined in cluster policy")
	cmd.Flags().StringSliceVarP(&saNames, "serviceaccount", "z", saNames, "service account in the current namespace to use as a user")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

// NewCmdRemoveRoleFromGroup implements the OpenShift cli remove-role-from-group command
func NewCmdRemoveRoleFromGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}
	roleNamespace := ""

	cmd := &cobra.Command{
		Use:   name + " ROLE GROUP [GROUP ...]",
		Short: "Remove a role from groups for the current project",
		Long:  `Remove a role from groups for the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, cmd, args, &options.Groups, "group", out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.checkRoleBindingNamespace(f, roleNamespace); err != nil {
				kcmdutil.CheckErr(err)
			}

			if err := options.RemoveRole(); err != nil {
				kcmdutil.CheckErr(err)
				return
			}
			if len(options.Output) == 0 {
				printSuccessForCommand(options.RoleName, false, "group", options.Targets, true, options.DryRun, out)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleBindingName, "rolebinding-name", "", "Name of the rolebinding to modify. If left empty it will operate on all rolebindings")
	cmd.Flags().StringVar(&roleNamespace, "role-namespace", "", "namespace where the role is located: empty means a role defined in cluster policy")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

// NewCmdRemoveRoleFromUser implements the OpenShift cli remove-role-from-user command
func NewCmdRemoveRoleFromUser(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{}
	saNames := []string{}
	roleNamespace := ""

	cmd := &cobra.Command{
		Use:   name + " ROLE USER [USER ...]",
		Short: "Remove a role from users for the current project",
		Long:  `Remove a role from users for the current project`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.CompleteUserWithSA(f, cmd, args, saNames, out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.checkRoleBindingNamespace(f, roleNamespace); err != nil {
				kcmdutil.CheckErr(err)
			}

			if err := options.RemoveRole(); err != nil {
				kcmdutil.CheckErr(err)
				return
			}
			if len(options.Output) == 0 {
				printSuccessForCommand(options.RoleName, false, "user", options.Targets, true, options.DryRun, out)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleBindingName, "rolebinding-name", "", "Name of the rolebinding to modify. If left empty it will operate on all rolebindings")
	cmd.Flags().StringVar(&roleNamespace, "role-namespace", "", "namespace where the role is located: empty means a role defined in cluster policy")
	cmd.Flags().StringSliceVarP(&saNames, "serviceaccount", "z", saNames, "service account in the current namespace to use as a user")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

// NewCmdAddClusterRoleToGroup implements the OpenShift cli add-cluster-role-to-group command
func NewCmdAddClusterRoleToGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{RoleKind: "ClusterRole"}

	cmd := &cobra.Command{
		Use:   name + " <role> <group> [group]...",
		Short: "Add a role to groups for all projects in the cluster",
		Long:  `Add a role to groups for all projects in the cluster`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, cmd, args, &options.Groups, "group", out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.AddRole(); err != nil {
				kcmdutil.CheckErr(err)
				return
			}
			if len(options.Output) == 0 {
				printSuccessForCommand(options.RoleName, true, "group", options.Targets, false, options.DryRun, out)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleBindingName, "rolebinding-name", "", "Name of the rolebinding to modify or create. If left empty creates a new rolebinding with a default name")
	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

// NewCmdAddClusterRoleToUser implements the OpenShift cli add-cluster-role-to-user command
func NewCmdAddClusterRoleToUser(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	saNames := []string{}
	options := &RoleModificationOptions{RoleKind: "ClusterRole"}

	cmd := &cobra.Command{
		Use:   name + " <role> <user | -z serviceaccount> [user]...",
		Short: "Add a role to users for all projects in the cluster",
		Long:  `Add a role to users for all projects in the cluster`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.CompleteUserWithSA(f, cmd, args, saNames, out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.AddRole(); err != nil {
				kcmdutil.CheckErr(err)
				return
			}
			if len(options.Output) == 0 {
				printSuccessForCommand(options.RoleName, true, "user", options.Targets, false, options.DryRun, out)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleBindingName, "rolebinding-name", "", "Name of the rolebinding to modify or create. If left empty creates a new rolebinding with a default name")
	cmd.Flags().StringSliceVarP(&saNames, "serviceaccount", "z", saNames, "service account in the current namespace to use as a user")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

// NewCmdRemoveClusterRoleFromGroup implements the OpenShift cli remove-cluster-role-from-group command
func NewCmdRemoveClusterRoleFromGroup(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &RoleModificationOptions{RoleKind: "ClusterRole"}

	cmd := &cobra.Command{
		Use:   name + " <role> <group> [group]...",
		Short: "Remove a role from groups for all projects in the cluster",
		Long:  `Remove a role from groups for all projects in the cluster`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, cmd, args, &options.Groups, "group", out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.RemoveRole(); err != nil {
				kcmdutil.CheckErr(err)
				return
			}
			if len(options.Output) == 0 {
				printSuccessForCommand(options.RoleName, false, "group", options.Targets, false, options.DryRun, out)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleBindingName, "rolebinding-name", "", "Name of the rolebinding to modify. If left empty it will operate on all rolebindings")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

// NewCmdRemoveClusterRoleFromUser implements the OpenShift cli remove-cluster-role-from-user command
func NewCmdRemoveClusterRoleFromUser(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	saNames := []string{}
	options := &RoleModificationOptions{RoleKind: "ClusterRole"}

	cmd := &cobra.Command{
		Use:   name + " <role> <user> [user]...",
		Short: "Remove a role from users for all projects in the cluster",
		Long:  `Remove a role from users for all projects in the cluster`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.CompleteUserWithSA(f, cmd, args, saNames, out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.RemoveRole(); err != nil {
				kcmdutil.CheckErr(err)
				return
			}
			if len(options.Output) == 0 {
				printSuccessForCommand(options.RoleName, false, "user", options.Targets, false, options.DryRun, out)
			}
		},
	}

	cmd.Flags().StringVar(&options.RoleBindingName, "rolebinding-name", "", "Name of the rolebinding to modify. If left empty it will operate on all rolebindings")
	cmd.Flags().StringSliceVarP(&saNames, "serviceaccount", "z", saNames, "service account in the current namespace to use as a user")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

func (o *RoleModificationOptions) checkRoleBindingNamespace(f *clientcmd.Factory, roleNamespace string) error {
	var err error
	o.RoleBindingNamespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}
	if len(roleNamespace) > 0 {
		if o.RoleBindingNamespace != roleNamespace {
			return fmt.Errorf("role binding in namespace %q can't reference role in different namespace %q",
				o.RoleBindingNamespace, roleNamespace)
		}
		o.RoleKind = "Role"
	} else {
		o.RoleKind = "ClusterRole"
	}
	return nil
}

func (o *RoleModificationOptions) innerComplete(f *clientcmd.Factory, cmd *cobra.Command, out io.Writer) error {
	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}
	o.RbacClient, err = rbacclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.DryRun = kcmdutil.GetFlagBool(cmd, "dry-run")
	o.Output = kcmdutil.GetFlagString(cmd, "output")
	o.PrintObj = func(obj runtime.Object) error {
		return kcmdutil.PrintObject(cmd, obj, out)
	}

	return nil
}

func (o *RoleModificationOptions) CompleteUserWithSA(f *clientcmd.Factory, cmd *cobra.Command, args []string, saNames []string, out io.Writer) error {
	if len(args) < 1 {
		return errors.New("you must specify a role")
	}

	o.RoleName = args[0]
	if len(args) > 1 {
		o.Users = append(o.Users, args[1:]...)
	}

	o.Targets = o.Users

	if (len(o.Users) == 0) && (len(saNames) == 0) {
		return errors.New("you must specify at least one user or service account")
	}

	// return an error if a fully-qualified service-account name is used
	for _, sa := range saNames {
		if strings.HasPrefix(sa, "system:serviceaccount") {
			return errors.New("--serviceaccount (-z) should only be used with short-form serviceaccount names (e.g. `default`)")
		}

		if errCauses := validation.ValidateServiceAccountName(sa, false); len(errCauses) > 0 {
			message := fmt.Sprintf("%q is not a valid serviceaccount name:\n  ", sa)
			message += strings.Join(errCauses, "\n  ")
			return errors.New(message)
		}
	}

	err := o.innerComplete(f, cmd, out)
	if err != nil {
		return err
	}

	defaultNamespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	for _, sa := range saNames {
		o.Targets = append(o.Targets, sa)
		o.Subjects = append(o.Subjects, rbac.Subject{Namespace: defaultNamespace, Name: sa, Kind: rbac.ServiceAccountKind})
	}

	return nil
}

func (o *RoleModificationOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, target *[]string, targetName string, out io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("you must specify at least two arguments: <role> <%s> [%s]...", targetName, targetName)
	}

	o.RoleName = args[0]
	*target = append(*target, args[1:]...)

	o.Targets = *target

	return o.innerComplete(f, cmd, out)
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

	roleBinding.SetSubjects(addSubjects(o.Users, o.Groups, o.Subjects, roleBinding.Subjects()))

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

func addSubjects(users []string, groups []string, subjects []rbac.Subject, existingSubjects []rbac.Subject) []rbac.Subject {
	subjectsToAdd := authorizationutil.BuildRBACSubjects(users, groups)
	subjectsToAdd = append(subjectsToAdd, subjects...)
	newSubjects := make([]rbac.Subject, len(existingSubjects))
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
		var resultingSubjects []rbac.Subject
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
			updatedBindings := &rbac.RoleBindingList{
				TypeMeta: metav1.TypeMeta{
					Kind:       "List",
					APIVersion: "v1",
				},
				ListMeta: metav1.ListMeta{},
			}
			for _, binding := range roleBindings {
				updatedBindings.Items = append(updatedBindings.Items, *(binding.Object().(*rbac.RoleBinding)))
			}
			updated = updatedBindings
		} else {
			updatedBindings := &rbac.ClusterRoleBindingList{
				TypeMeta: metav1.TypeMeta{
					Kind:       "List",
					APIVersion: "v1",
				},
				ListMeta: metav1.ListMeta{},
			}
			for _, binding := range roleBindings {
				updatedBindings.Items = append(updatedBindings.Items, *(binding.Object().(*rbac.ClusterRoleBinding)))
			}
			updated = updatedBindings
		}

		return o.PrintObj(updated)
	}

	if o.DryRun {
		return nil
	}

	for _, roleBinding := range roleBindings {
		if len(roleBinding.Subjects()) > 0 || roleBinding.Annotation(rbac.AutoUpdateAnnotationKey) == "false" {
			err = roleBinding.Update()
		} else {
			err = roleBinding.Delete()
		}
		if err != nil {
			return err
		}
	}
	if found == 0 {
		return fmt.Errorf("unable to find target %v", o.Targets)
	}

	return nil
}

func removeSubjects(haystack, needles []rbac.Subject) ([]rbac.Subject, int) {
	newSubjects := []rbac.Subject{}
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
