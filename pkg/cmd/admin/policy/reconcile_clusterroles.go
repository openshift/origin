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
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const ReconcileClusterRolesRecommendedName = "reconcile-cluster-roles"

type reconcileClusterOptions struct {
	Confirmed bool

	Out io.Writer

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
  $ %[1]s --confirm`
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
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			changedClusterRoles, err := o.ChangedClusterRoles()
			kcmdutil.CheckErr(err)

			if len(changedClusterRoles) == 0 {
				return
			}

			if (len(kcmdutil.GetFlagString(cmd, "output")) != 0) && !o.Confirmed {
				list := &kapi.List{}
				for _, item := range changedClusterRoles {
					list.Items = append(list.Items, item)
				}

				kcmdutil.CheckErr(f.Factory.PrintObject(cmd, list, out))
			}

			if o.Confirmed {
				kcmdutil.CheckErr(o.ReplaceChangedRoles(changedClusterRoles))
			}
		},
	}

	cmd.Flags().BoolVar(&o.Confirmed, "confirm", o.Confirmed, "Specify that cluster roles should be modified. Defaults to false, displaying what would be replaced but not actually replacing anything.")
	kcmdutil.AddPrinterFlags(cmd)
	cmd.Flags().Lookup("output").DefValue = "yaml"
	cmd.Flags().Lookup("output").Value.Set("yaml")

	return cmd
}

func (o *reconcileClusterOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	if len(args) != 0 {
		return errors.New("No arguments are allowed")
	}

	oclient, _, err := f.Clients()
	if err != nil {
		return err
	}
	o.RoleClient = oclient.ClusterRoles()

	return nil
}

func (o *reconcileClusterOptions) ReplaceChangedRoles(changedRoles []*authorizationapi.ClusterRole) error {
	for i := range changedRoles {
		role, err := o.RoleClient.Get(changedRoles[i].Name)
		if err != nil && !kapierrors.IsNotFound(err) {
			return err
		}

		if kapierrors.IsNotFound(err) {
			createdRole, createErr := o.RoleClient.Create(changedRoles[i])
			if createErr != nil {
				return createErr
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
			changedRoles = append(changedRoles, expectedClusterRole)
		}
	}

	return changedRoles, nil
}
