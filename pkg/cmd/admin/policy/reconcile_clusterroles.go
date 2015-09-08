package policy

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// ReconcileClusterRolesRecommendedName is the recommended command name
const ReconcileClusterRolesRecommendedName = "reconcile-cluster-roles"

type reconcileClusterOptions struct {
	Confirmed bool
	Union     bool

	Out    io.Writer
	Output string

	RoleClient client.ClusterRoleInterface
}

const (
	reconcileLong = `
Replace cluster roles to match the recommended bootstrap policy

This command will inspect the cluster roles against the recommended bootstrap policy.  Any cluster role
that does not match will be replaced by the recommended bootstrap role.  This command will not remove
any additional cluster role.

You can see which cluster role have recommended changed by choosing an output type.`

	reconcileExample = `  // Display the cluster roles that would be modified
  $ %[1]s

  // Replace cluster roles that don't match the current defaults
  $ %[1]s --confirm

  // Display the union of the default and modified cluster roles
  $ %[1]s --additive-only`
)

// NewCmdReconcileClusterRoles implements the OpenShift cli reconcile-cluster-roles command
func NewCmdReconcileClusterRoles(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &reconcileClusterOptions{Out: out}

	cmd := &cobra.Command{
		Use:     name,
		Short:   "Replace cluster roles to match the recommended bootstrap policy",
		Long:    reconcileLong,
		Example: fmt.Sprintf(reconcileExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := o.Complete(cmd, f, args); err != nil {
				kcmdutil.CheckErr(err)
			}

			if err := o.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := o.RunReconcileClusterRoles(cmd, f); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().BoolVar(&o.Confirmed, "confirm", o.Confirmed, "Specify that cluster roles should be modified. Defaults to false, displaying what would be replaced but not actually replacing anything.")
	cmd.Flags().BoolVar(&o.Union, "additive-only", o.Union, "Preserves modified cluster roles.")
	kcmdutil.AddPrinterFlags(cmd)
	cmd.Flags().Lookup("output").DefValue = "yaml"
	cmd.Flags().Lookup("output").Value.Set("yaml")

	return cmd
}

func (o *reconcileClusterOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	if len(args) != 0 {
		return kcmdutil.UsageError(cmd, "no arguments are allowed")
	}

	oclient, _, err := f.Clients()
	if err != nil {
		return err
	}
	o.RoleClient = oclient.ClusterRoles()

	o.Output = kcmdutil.GetFlagString(cmd, "output")

	return nil
}

func (o *reconcileClusterOptions) Validate() error {
	if o.RoleClient == nil {
		return errors.New("a role client is required")
	}
	if o.Output != "yaml" && o.Output != "json" && o.Output != "" {
		return fmt.Errorf("unknown output specified: %s", o.Output)
	}
	return nil
}

// RunReconcileClusterRoles contains all the necessary functionality for the OpenShift cli reconcile-cluster-roles command
func (o *reconcileClusterOptions) RunReconcileClusterRoles(cmd *cobra.Command, f *clientcmd.Factory) error {
	changedClusterRoles, err := o.ChangedClusterRoles()
	if err != nil {
		return err
	}

	if len(changedClusterRoles) == 0 {
		return nil
	}

	if (len(o.Output) != 0) && !o.Confirmed {
		list := &kapi.List{}
		for _, item := range changedClusterRoles {
			list.Items = append(list.Items, item)
		}

		if err := f.Factory.PrintObject(cmd, list, o.Out); err != nil {
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
func (o *reconcileClusterOptions) ChangedClusterRoles() ([]*authorizationapi.ClusterRole, error) {
	changedRoles := []*authorizationapi.ClusterRole{}

	bootstrapClusterRoles := bootstrappolicy.GetBootstrapClusterRoles()
	for i := range bootstrapClusterRoles {
		expectedClusterRole := &bootstrapClusterRoles[i]

		actualClusterRole, err := o.RoleClient.Get(expectedClusterRole.Name)
		if kapierrors.IsNotFound(err) {
			changedRoles = append(changedRoles, expectedClusterRole)
			continue
		}
		if err != nil {
			return nil, err
		}

		if !kapi.Semantic.DeepEqual(expectedClusterRole.Rules, actualClusterRole.Rules) {
			if o.Union {
				_, missingRules := rulevalidation.Covers(expectedClusterRole.Rules, actualClusterRole.Rules)
				expectedClusterRole.Rules = append(expectedClusterRole.Rules, missingRules...)
			}
			changedRoles = append(changedRoles, expectedClusterRole)
		}
	}

	return changedRoles, nil
}

// ReplaceChangedRoles will reconcile all the changed roles back to the recommended bootstrap policy
func (o *reconcileClusterOptions) ReplaceChangedRoles(changedRoles []*authorizationapi.ClusterRole) error {
	for i := range changedRoles {
		role, err := o.RoleClient.Get(changedRoles[i].Name)
		if err != nil && !kapierrors.IsNotFound(err) {
			return err
		}

		if kapierrors.IsNotFound(err) {
			createdRole, err := o.RoleClient.Create(changedRoles[i])
			if err != nil {
				return err
			}

			fmt.Fprintf(o.Out, "clusterrole/%s\n", createdRole.Name)
			continue
		}

		role.Rules = changedRoles[i].Rules
		updatedRole, err := o.RoleClient.Update(role)
		if err != nil {
			return err
		}

		fmt.Fprintf(o.Out, "clusterrole/%s\n", updatedRole.Name)
	}

	return nil
}
