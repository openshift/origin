package policy

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/rbac"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationtypedclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	"github.com/openshift/origin/pkg/authorization/registry/util"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	osutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

// ReconcileProtectAnnotation is the name of an annotation which prevents reconciliation if set to "true"
const ReconcileProtectAnnotation = "openshift.io/reconcile-protect"

// ReconcileClusterRolesRecommendedName is the recommended command name
const ReconcileClusterRolesRecommendedName = "reconcile-cluster-roles"

type ReconcileClusterRolesOptions struct {
	// RolesToReconcile says which roles should be reconciled.  An empty or nil slice means
	// reconcile all of them.
	RolesToReconcile []string

	Confirmed bool
	Union     bool

	Out    io.Writer
	ErrOut io.Writer
	Output string

	RoleClient authorizationtypedclient.ClusterRoleInterface
}

var (
	reconcileLong = templates.LongDesc(`
		Update cluster roles to match the recommended bootstrap policy

		This command will compare cluster roles against the recommended bootstrap policy.  Any cluster role
		that does not match will be replaced by the recommended bootstrap role.  This command will not remove
		any additional cluster role.

		Cluster roles with the annotation %s set to "true" are skipped.

		You can see which cluster roles have recommended changed by choosing an output type.`)

	reconcileExample = templates.Examples(`
		# Display the names of cluster roles that would be modified
	  %[1]s -o name

	  # Add missing permissions to cluster roles that don't match the current defaults
	  %[1]s --confirm

	  # Add missing permissions and remove extra permissions from
	  # cluster roles that don't match the current defaults
	  %[1]s --additive-only=false --confirm

	  # Display the union of the default and modified cluster roles
	  %[1]s --additive-only`)
)

// NewCmdReconcileClusterRoles implements the OpenShift cli reconcile-cluster-roles command
func NewCmdReconcileClusterRoles(name, fullName string, f *clientcmd.Factory, out, errout io.Writer) *cobra.Command {
	o := &ReconcileClusterRolesOptions{
		Out:    out,
		ErrOut: errout,
		Union:  true,
	}

	cmd := &cobra.Command{
		Use:     name + " [ClusterRoleName]...",
		Short:   "Update cluster roles to match the recommended bootstrap policy",
		Long:    fmt.Sprintf(reconcileLong, ReconcileProtectAnnotation),
		Example: fmt.Sprintf(reconcileExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := o.Complete(cmd, f, args); err != nil {
				kcmdutil.CheckErr(err)
			}

			if err := o.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := o.RunReconcileClusterRoles(cmd, f); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
		Deprecated: fmt.Sprintf("use 'oc auth reconcile'"),
	}

	cmd.Flags().BoolVar(&o.Confirmed, "confirm", o.Confirmed, "If true, specify that cluster roles should be modified. Defaults to false, displaying what would be replaced but not actually replacing anything.")
	cmd.Flags().BoolVar(&o.Union, "additive-only", o.Union, "If true, preserves modified cluster roles.")
	kcmdutil.AddPrinterFlags(cmd)
	cmd.Flags().Lookup("output").DefValue = "yaml"
	cmd.Flags().Lookup("output").Value.Set("yaml")

	return cmd
}

func (o *ReconcileClusterRolesOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	authorizationClient, err := f.OpenshiftInternalAuthorizationClient()
	if err != nil {
		return err
	}
	o.RoleClient = authorizationClient.Authorization().ClusterRoles()

	o.Output = kcmdutil.GetFlagString(cmd, "output")

	mapper, _ := f.Object()
	for _, resourceString := range args {
		resource, name, err := osutil.ResolveResource(authorizationapi.Resource("clusterroles"), resourceString, mapper)
		if err != nil {
			return err
		}
		if !authorizationapi.IsResourceOrLegacy("clusterroles", resource) {
			return fmt.Errorf("%v is not a valid resource type for this command", resource)
		}
		if len(name) == 0 {
			return fmt.Errorf("%s did not contain a name", resourceString)
		}

		o.RolesToReconcile = append(o.RolesToReconcile, name)
	}

	return nil
}

func (o *ReconcileClusterRolesOptions) Validate() error {
	if o.RoleClient == nil {
		return errors.New("a role client is required")
	}
	return nil
}

// RunReconcileClusterRoles contains all the necessary functionality for the OpenShift cli reconcile-cluster-roles command
func (o *ReconcileClusterRolesOptions) RunReconcileClusterRoles(cmd *cobra.Command, f *clientcmd.Factory) error {
	changedClusterRoles, skippedClusterRoles, err := o.ChangedClusterRoles()
	if err != nil {
		return err
	}

	if len(skippedClusterRoles) > 0 {
		fmt.Fprintf(o.ErrOut, "Skipped reconciling roles with the annotation %s=true\n", ReconcileProtectAnnotation)
		for _, role := range skippedClusterRoles {
			fmt.Fprintf(o.ErrOut, "skipped: clusterrole/%s\n", role.Name)
		}
	}

	if len(changedClusterRoles) == 0 {
		return nil
	}

	if (len(o.Output) != 0) && !o.Confirmed {
		list := &kapi.List{}
		for _, item := range changedClusterRoles {
			list.Items = append(list.Items, item)
		}
		mapper, _ := f.Object()
		fn := cmdutil.VersionedPrintObject(f.PrintObject, cmd, mapper, o.Out)
		if err := fn(list); err != nil {
			return err
		}
	}

	if o.Confirmed {
		return o.ReplaceChangedRoles(changedClusterRoles)
	}

	return nil
}

// ChangedClusterRoles returns the roles that must be created and/or updated to
// match the recommended bootstrap policy
func (o *ReconcileClusterRolesOptions) ChangedClusterRoles() ([]*rbac.ClusterRole, []*rbac.ClusterRole, error) {
	changedRoles := []*rbac.ClusterRole{}
	skippedRoles := []*rbac.ClusterRole{}

	rolesToReconcile := sets.NewString(o.RolesToReconcile...)
	rolesNotFound := sets.NewString(o.RolesToReconcile...)
	bootstrapClusterRoles := bootstrappolicy.GetBootstrapClusterRoles()
	for i := range bootstrapClusterRoles {
		expectedClusterRole := &bootstrapClusterRoles[i]
		if (len(rolesToReconcile) > 0) && !rolesToReconcile.Has(expectedClusterRole.Name) {
			continue
		}
		rolesNotFound.Delete(expectedClusterRole.Name)

		actualClusterRole, err := o.RoleClient.Get(expectedClusterRole.Name, metav1.GetOptions{})
		if kapierrors.IsNotFound(err) {
			changedRoles = append(changedRoles, expectedClusterRole)
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		actualRBACClusterRole, err := util.ClusterRoleToRBAC(actualClusterRole)
		if err != nil {
			return nil, nil, err
		}

		if reconciledClusterRole, needsReconciliation := computeReconciledRole(*expectedClusterRole, *actualRBACClusterRole, o.Union); needsReconciliation {
			if actualClusterRole.Annotations[ReconcileProtectAnnotation] == "true" {
				skippedRoles = append(skippedRoles, reconciledClusterRole)
			} else {
				changedRoles = append(changedRoles, reconciledClusterRole)
			}
		}
	}

	if len(rolesNotFound) != 0 {
		// return the known changes and the error so that a caller can decide if he wants a partial update
		return changedRoles, skippedRoles, fmt.Errorf("did not find requested cluster role %s", rolesNotFound.List())
	}

	return changedRoles, skippedRoles, nil
}

func computeReconciledRole(expected rbac.ClusterRole, actual rbac.ClusterRole, union bool) (*rbac.ClusterRole, bool) {
	existingAnnotationKeys := sets.StringKeySet(actual.Annotations)
	expectedAnnotationKeys := sets.StringKeySet(expected.Annotations)
	missingAnnotationKeys := !existingAnnotationKeys.HasAll(expectedAnnotationKeys.List()...)

	// Copy any existing labels, so the displayed update is correct
	// This assumes bootstrap roles will not set any labels
	// These labels aren't actually used during update; the latest labels are pulled from the existing object again
	// Annotations are merged in a way that guarantees that user made changes have precedence over the defaults
	// The latest annotations are pulled from the existing object again during update before doing the actual merge
	expected.Labels = actual.Labels
	expected.Annotations = mergeAnnotations(expected.Annotations, actual.Annotations)

	_, extraRules := rbacregistryvalidation.Covers(expected.Rules, actual.Rules)
	_, missingRules := rbacregistryvalidation.Covers(actual.Rules, expected.Rules)

	// We need to reconcile:
	// 1. if we're missing rules
	// 2. if there are extra rules we need to remove
	// 3. if we are missing annotations
	needsReconciliation := (len(missingRules) > 0) || (!union && len(extraRules) > 0) || missingAnnotationKeys

	if !needsReconciliation {
		return nil, false
	}

	if union {
		expected.Rules = append(expected.Rules, extraRules...)
	}
	return &expected, true
}

// ReplaceChangedRoles will reconcile all the changed roles back to the recommended bootstrap policy
func (o *ReconcileClusterRolesOptions) ReplaceChangedRoles(changedRoles []*rbac.ClusterRole) error {
	errs := []error{}
	for i := range changedRoles {
		role, err := o.RoleClient.Get(changedRoles[i].Name, metav1.GetOptions{})
		if err != nil && !kapierrors.IsNotFound(err) {
			errs = append(errs, err)
			continue
		}

		if kapierrors.IsNotFound(err) {
			role, err := util.ClusterRoleFromRBAC(changedRoles[i])
			if err != nil {
				errs = append(errs, err)
				continue
			}
			createdRole, err := o.RoleClient.Create(role)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			fmt.Fprintf(o.Out, "clusterrole/%s\n", createdRole.Name)
			continue
		}

		rbacRole, err := util.ClusterRoleToRBAC(role)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		rbacRole.Rules = changedRoles[i].Rules
		rbacRole.Annotations = mergeAnnotations(changedRoles[i].Annotations, rbacRole.Annotations)

		role, err = util.ClusterRoleFromRBAC(rbacRole)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		updatedRole, err := o.RoleClient.Update(role)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		fmt.Fprintf(o.Out, "clusterrole/%s\n", updatedRole.Name)
	}

	return kerrors.NewAggregate(errs)
}

// mergeAnnotations combines the given annotation maps with the later annotations having higher precedence
func mergeAnnotations(maps ...map[string]string) map[string]string {
	output := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			output[k] = v
		}
	}
	return output
}
