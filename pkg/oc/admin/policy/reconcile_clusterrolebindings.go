package policy

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	"k8s.io/kubernetes/pkg/apis/rbac"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationclientinternal "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	authorizationtypedclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	"github.com/openshift/origin/pkg/authorization/registry/util"
	authorizationutil "github.com/openshift/origin/pkg/authorization/util"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/print"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

// ReconcileClusterRoleBindingsRecommendedName is the recommended command name
const ReconcileClusterRoleBindingsRecommendedName = "reconcile-cluster-role-bindings"

// ReconcileClusterRoleBindingsOptions contains all the necessary functionality for the OpenShift cli reconcile-cluster-role-bindings command
type ReconcileClusterRoleBindingsOptions struct {
	// RolesToReconcile says which roles should have their default bindings reconciled.
	// An empty or nil slice means reconcile all of them.
	RolesToReconcile []string

	Confirmed bool
	Union     bool

	ExcludeSubjects []rbac.Subject

	Out    io.Writer
	Err    io.Writer
	Output string

	RoleBindingClient authorizationtypedclient.ClusterRoleBindingInterface
}

var (
	reconcileBindingsLong = templates.LongDesc(`
		Update cluster role bindings to match the recommended bootstrap policy

		This command will inspect the cluster role bindings against the recommended bootstrap policy.
		Any cluster role binding that does not match will be replaced by the recommended bootstrap role binding.
		This command will not remove any additional cluster role bindings.

		You can see which recommended cluster role bindings have changed by choosing an output type.`)

	reconcileBindingsExample = templates.Examples(`
		# Display the names of cluster role bindings that would be modified
	  %[1]s -o name

	  # Display the cluster role bindings that would be modified, removing any extra subjects
	  %[1]s --additive-only=false

	  # Update cluster role bindings that don't match the current defaults
	  %[1]s --confirm

	  # Update cluster role bindings that don't match the current defaults, avoid adding roles to the system:authenticated group
	  %[1]s --confirm --exclude-groups=system:authenticated

	  # Update cluster role bindings that don't match the current defaults, removing any extra subjects from the binding
	  %[1]s --confirm --additive-only=false`)
)

// NewCmdReconcileClusterRoleBindings implements the OpenShift cli reconcile-cluster-role-bindings command
func NewCmdReconcileClusterRoleBindings(name, fullName string, f *clientcmd.Factory, out, err io.Writer) *cobra.Command {
	o := &ReconcileClusterRoleBindingsOptions{
		Out:   out,
		Err:   err,
		Union: true,
	}

	excludeUsers := []string{}
	excludeGroups := []string{}

	cmd := &cobra.Command{
		Use:     name + " [ClusterRoleName]...",
		Short:   "Update cluster role bindings to match the recommended bootstrap policy",
		Long:    reconcileBindingsLong,
		Example: fmt.Sprintf(reconcileBindingsExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := o.Complete(cmd, f, args, excludeUsers, excludeGroups); err != nil {
				kcmdutil.CheckErr(err)
			}

			if err := o.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := o.RunReconcileClusterRoleBindings(cmd, f); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
		Deprecated: "use 'oc auth reconcile'",
	}

	cmd.Flags().BoolVar(&o.Confirmed, "confirm", o.Confirmed, "If true, specify that cluster role bindings should be modified. Defaults to false, displaying what would be replaced but not actually replacing anything.")
	cmd.Flags().BoolVar(&o.Union, "additive-only", o.Union, "If true, preserves extra subjects in cluster role bindings.")
	cmd.Flags().StringSliceVar(&excludeUsers, "exclude-users", excludeUsers, "Do not add cluster role bindings for these user names.")
	cmd.Flags().StringSliceVar(&excludeGroups, "exclude-groups", excludeGroups, "Do not add cluster role bindings for these group names.")
	kcmdutil.AddPrinterFlags(cmd)
	cmd.Flags().Lookup("output").DefValue = "yaml"
	cmd.Flags().Lookup("output").Value.Set("yaml")

	return cmd
}

func (o *ReconcileClusterRoleBindingsOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string, excludeUsers, excludeGroups []string) error {
	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}
	authorizationClient, err := authorizationclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	o.RoleBindingClient = authorizationClient.Authorization().ClusterRoleBindings()

	o.Output = kcmdutil.GetFlagString(cmd, "output")

	o.ExcludeSubjects = authorizationutil.BuildRBACSubjects(excludeUsers, excludeGroups)

	mapper, _ := f.Object()
	for _, resourceString := range args {
		resource, name, err := cmdutil.ResolveResource(authorizationapi.Resource("clusterroles"), resourceString, mapper)
		if err != nil {
			return err
		}
		if resource != authorizationapi.Resource("clusterroles") {
			return fmt.Errorf("%v is not a valid resource type for this command", resource)
		}
		if len(name) == 0 {
			return fmt.Errorf("%s did not contain a name", resourceString)
		}

		o.RolesToReconcile = append(o.RolesToReconcile, name)
	}

	return nil
}

func (o *ReconcileClusterRoleBindingsOptions) Validate() error {
	if o.RoleBindingClient == nil {
		return errors.New("a role binding client is required")
	}
	return nil
}

func (o *ReconcileClusterRoleBindingsOptions) RunReconcileClusterRoleBindings(cmd *cobra.Command, f *clientcmd.Factory) error {
	changedClusterRoleBindings, skippedClusterRoleBindings, fetchErr := o.ChangedClusterRoleBindings()
	if fetchErr != nil && !IsClusterRoleBindingLookupError(fetchErr) {
		// we got an error that isn't due to a partial match, so we can't continue
		return fetchErr
	}

	if len(skippedClusterRoleBindings) > 0 {
		fmt.Fprintf(o.Err, "Skipped reconciling roles with the annotation %s=true\n", ReconcileProtectAnnotation)
		for _, role := range skippedClusterRoleBindings {
			fmt.Fprintf(o.Err, "skipped: clusterrolebinding/%s\n", role.Name)
		}
	}

	if len(changedClusterRoleBindings) == 0 {
		return fetchErr
	}

	errs := []error{}
	if fetchErr != nil {
		errs = append(errs, fetchErr)
	}

	if (len(o.Output) != 0) && !o.Confirmed {
		list := &kapi.List{}
		for _, item := range changedClusterRoleBindings {
			list.Items = append(list.Items, item)
		}
		fn := print.VersionedPrintObject(legacyscheme.Scheme, legacyscheme.Registry, kcmdutil.PrintObject, cmd, o.Out)
		if err := fn(list); err != nil {
			errs = append(errs, err)
			return kutilerrors.NewAggregate(errs)
		}
	}

	if o.Confirmed {
		if err := o.ReplaceChangedRoleBindings(changedClusterRoleBindings); err != nil {
			errs = append(errs, err)
			return kutilerrors.NewAggregate(errs)
		}
	}

	return fetchErr
}

// ChangedClusterRoleBindings returns the role bindings that must be created and/or updated to
// match the recommended bootstrap policy. If roles to reconcile are provided, but not all are
// found, all partial results are returned.
func (o *ReconcileClusterRoleBindingsOptions) ChangedClusterRoleBindings() ([]*rbac.ClusterRoleBinding, []*rbac.ClusterRoleBinding, error) {
	changedRoleBindings := []*rbac.ClusterRoleBinding{}
	skippedRoleBindings := []*rbac.ClusterRoleBinding{}

	rolesToReconcile := sets.NewString(o.RolesToReconcile...)
	rolesNotFound := sets.NewString(o.RolesToReconcile...)
	bootstrapClusterRoleBindings := bootstrappolicy.GetBootstrapClusterRoleBindings()
	for i := range bootstrapClusterRoleBindings {
		expectedClusterRoleBinding := &bootstrapClusterRoleBindings[i]
		if (len(rolesToReconcile) > 0) && !rolesToReconcile.Has(expectedClusterRoleBinding.RoleRef.Name) {
			continue
		}
		rolesNotFound.Delete(expectedClusterRoleBinding.RoleRef.Name)

		actualClusterRoleBinding, err := o.RoleBindingClient.Get(expectedClusterRoleBinding.Name, metav1.GetOptions{})
		if kapierrors.IsNotFound(err) {
			// Remove excluded subjects from the new role binding
			expectedClusterRoleBinding.Subjects, _ = DiffSubjects(expectedClusterRoleBinding.Subjects, o.ExcludeSubjects)
			changedRoleBindings = append(changedRoleBindings, expectedClusterRoleBinding)
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		actualRBACClusterRoleBinding, err := util.ClusterRoleBindingToRBAC(actualClusterRoleBinding)
		if err != nil {
			return nil, nil, err
		}

		// Copy any existing labels/annotations, so the displayed update is correct
		// This assumes bootstrap role bindings will not set any labels/annotations
		// These aren't actually used during update; the latest labels/annotations are pulled from the existing object again
		expectedClusterRoleBinding.Labels = actualRBACClusterRoleBinding.Labels
		expectedClusterRoleBinding.Annotations = actualRBACClusterRoleBinding.Annotations

		if updatedClusterRoleBinding, needsUpdating := computeUpdatedBinding(*expectedClusterRoleBinding, *actualRBACClusterRoleBinding, o.ExcludeSubjects, o.Union); needsUpdating {
			if actualClusterRoleBinding.Annotations[ReconcileProtectAnnotation] == "true" {
				skippedRoleBindings = append(skippedRoleBindings, updatedClusterRoleBinding)
			} else {
				changedRoleBindings = append(changedRoleBindings, updatedClusterRoleBinding)
			}
		}
	}

	if len(rolesNotFound) != 0 {
		// return the known changes and the error so that a caller can decide if he wants a partial update
		return changedRoleBindings, skippedRoleBindings, NewClusterRoleBindingLookupError(rolesNotFound.List())
	}

	return changedRoleBindings, skippedRoleBindings, nil
}

// ReplaceChangedRoleBindings will reconcile all the changed system role bindings back to the recommended bootstrap policy
func (o *ReconcileClusterRoleBindingsOptions) ReplaceChangedRoleBindings(changedRoleBindings []*rbac.ClusterRoleBinding) error {
	errs := []error{}
	for i := range changedRoleBindings {
		roleBinding, err := o.RoleBindingClient.Get(changedRoleBindings[i].Name, metav1.GetOptions{})
		if err != nil && !kapierrors.IsNotFound(err) {
			errs = append(errs, err)
			continue
		}

		if kapierrors.IsNotFound(err) {
			roleBinding, err := util.ClusterRoleBindingFromRBAC(changedRoleBindings[i])
			if err != nil {
				errs = append(errs, err)
				continue
			}
			createdRoleBinding, err := o.RoleBindingClient.Create(roleBinding)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			fmt.Fprintf(o.Out, "clusterrolebinding/%s\n", createdRoleBinding.Name)
			continue
		}
		rbacRoleBinding, err := util.ClusterRoleBindingToRBAC(roleBinding)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// RoleRef is immutable, to reset this, we have to delete/recreate
		if !kapihelper.Semantic.DeepEqual(roleBinding.RoleRef, changedRoleBindings[i].RoleRef) {
			rbacRoleBinding.RoleRef = changedRoleBindings[i].RoleRef
			rbacRoleBinding.Subjects = changedRoleBindings[i].Subjects

			// TODO: for extra credit, determine whether the right to delete/create this rolebinding for the current user came from this rolebinding before deleting it
			err := o.RoleBindingClient.Delete(roleBinding.Name, nil)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			roleBinding, err := util.ClusterRoleBindingFromRBAC(changedRoleBindings[i])
			if err != nil {
				errs = append(errs, err)
				continue
			}
			createdRoleBinding, err := o.RoleBindingClient.Create(roleBinding)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			fmt.Fprintf(o.Out, "clusterrolebinding/%s\n", createdRoleBinding.Name)
			continue
		}

		rbacRoleBinding.Subjects = changedRoleBindings[i].Subjects
		roleBinding, err = util.ClusterRoleBindingFromRBAC(rbacRoleBinding)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		updatedRoleBinding, err := o.RoleBindingClient.Update(roleBinding)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		fmt.Fprintf(o.Out, "clusterrolebinding/%s\n", updatedRoleBinding.Name)
	}

	return kutilerrors.NewAggregate(errs)
}

func computeUpdatedBinding(expected rbac.ClusterRoleBinding, actual rbac.ClusterRoleBinding, excludeSubjects []rbac.Subject, union bool) (*rbac.ClusterRoleBinding, bool) {
	needsUpdating := false

	// Always reset the roleref if it is different
	if !kapihelper.Semantic.DeepEqual(expected.RoleRef, actual.RoleRef) {
		needsUpdating = true
	}

	// compute the list of subjects we should not add roles for (existing subjects in the exclude list should be preserved)
	doNotAddSubjects, _ := DiffSubjects(excludeSubjects, actual.Subjects)
	// remove any excluded subjects that do not exist from our expected subject list (so we don't add them)
	expectedSubjects, _ := DiffSubjects(expected.Subjects, doNotAddSubjects)

	missingSubjects, extraSubjects := DiffSubjects(expectedSubjects, actual.Subjects)
	// Always add missing expected subjects
	if len(missingSubjects) > 0 {
		needsUpdating = true
	}
	// extra subjects only require a change if we're not unioning
	if len(extraSubjects) > 0 && !union {
		needsUpdating = true
	}

	if !needsUpdating {
		return nil, false
	}

	updated := expected
	updated.Subjects = expectedSubjects
	if union {
		updated.Subjects = append(updated.Subjects, extraSubjects...)
	}
	return &updated, true
}

func contains(list []rbac.Subject, item rbac.Subject) bool {
	for _, listItem := range list {
		if kapihelper.Semantic.DeepEqual(listItem, item) {
			return true
		}
	}
	return false
}

// DiffSubjects returns lists containing the items unique to each provided list:
//   list1Only = list1 - list2
//   list2Only = list2 - list1
// if both returned lists are empty, the provided lists are equal
func DiffSubjects(list1 []rbac.Subject, list2 []rbac.Subject) (list1Only []rbac.Subject, list2Only []rbac.Subject) {
	for _, list1Item := range list1 {
		if !contains(list2, list1Item) {
			if !contains(list1Only, list1Item) {
				list1Only = append(list1Only, list1Item)
			}
		}
	}
	for _, list2Item := range list2 {
		if !contains(list1, list2Item) {
			if !contains(list2Only, list2Item) {
				list2Only = append(list2Only, list2Item)
			}
		}
	}
	return
}

func NewClusterRoleBindingLookupError(rolesNotFound []string) error {
	return &clusterRoleBindingLookupError{
		rolesNotFound: rolesNotFound,
	}
}

type clusterRoleBindingLookupError struct {
	rolesNotFound []string
}

func (e *clusterRoleBindingLookupError) Error() string {
	return fmt.Sprintf("did not find requested cluster roles: %s", strings.Join(e.rolesNotFound, ", "))
}

func IsClusterRoleBindingLookupError(err error) bool {
	if err == nil {
		return false
	}

	_, ok := err.(*clusterRoleBindingLookupError)
	return ok
}
