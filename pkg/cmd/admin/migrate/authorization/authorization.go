package authorization

import (
	"errors"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	rbacinternalversion "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/rbac/internalversion"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/controller/authorizationsync"
	"github.com/openshift/origin/pkg/cmd/admin/migrate"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"

	"github.com/spf13/cobra"
)

var (
	internalMigrateAuthorizationLong = templates.LongDesc(`
		Check for parity between Openshift authorization objects and Kubernetes RBAC

		A controller is used to keep Openshift authorization objects and Kubernetes RBAC in sync.
		This command checks for parity between those objects across all namespaces and reports
		all objects that are out of sync.  These objects require manual intervention to sync
		as the controller handles all cases where automatic sync is possible.

		The following resource types are checked by this command:

		* clusterrole
		* role
		* clusterrolebinding
		* rolebinding

		No resources are mutated.`)

	errOutOfSync = errors.New("is not in sync with RBAC")
)

type MigrateAuthorizationOptions struct {
	migrate.ResourceOptions
	rbac rbacinternalversion.RbacInterface
}

func NewCmdMigrateAuthorization(name, fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	options := &MigrateAuthorizationOptions{
		ResourceOptions: migrate.ResourceOptions{
			In:            in,
			Out:           out,
			ErrOut:        errout,
			AllNamespaces: true,
			Include: []string{
				"clusterroles.authorization.openshift.io",
				"roles.authorization.openshift.io",
				"clusterrolebindings.authorization.openshift.io",
				"rolebindings.authorization.openshift.io",
			},
		},
	}
	cmd := &cobra.Command{
		Use:   name,
		Short: "Confirm that Origin authorization resources are in sync with their RBAC equivalents",
		Long:  internalMigrateAuthorizationLong,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(name, f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			kcmdutil.CheckErr(options.Run())
		},
	}
	return cmd
}

func (o *MigrateAuthorizationOptions) Complete(name string, f *clientcmd.Factory, c *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("%s takes no positional arguments", name)
	}

	if err := o.ResourceOptions.Complete(f, c); err != nil {
		return err
	}

	_, kclient, err := f.Clients()
	if err != nil {
		return err
	}
	o.rbac = kclient.Rbac()

	return nil
}

func (o MigrateAuthorizationOptions) Validate() error {
	return o.ResourceOptions.Validate()
}

func (o MigrateAuthorizationOptions) Run() error {
	return o.ResourceOptions.Visitor().Visit(func(info *resource.Info) (migrate.Reporter, error) {
		return o.checkParity(info.Object)
	})
}

// checkParity confirms that Openshift authorization objects are in sync with Kubernetes RBAC
// and returns an error if they are out of sync or if it encounters a conversion error
func (o *MigrateAuthorizationOptions) checkParity(obj runtime.Object) (migrate.Reporter, error) {
	var errlist []error
	switch t := obj.(type) {
	case *authorizationapi.ClusterRole:
		errlist = append(errlist, o.checkClusterRole(t)...)
	case *authorizationapi.Role:
		errlist = append(errlist, o.checkRole(t)...)
	case *authorizationapi.ClusterRoleBinding:
		errlist = append(errlist, o.checkClusterRoleBinding(t)...)
	case *authorizationapi.RoleBinding:
		errlist = append(errlist, o.checkRoleBinding(t)...)
	default:
		return nil, nil // indicate that we ignored the object
	}
	return migrate.NotChanged, utilerrors.NewAggregate(errlist) // we only perform read operations
}

func (o *MigrateAuthorizationOptions) checkClusterRole(originClusterRole *authorizationapi.ClusterRole) []error {
	var errlist []error

	// convert the origin role to a rbac role
	convertedClusterRole, err := authorizationsync.ConvertToRBACClusterRole(originClusterRole)
	if err != nil {
		errlist = append(errlist, err)
	}

	// try to get the equivalent rbac role from the api
	rbacClusterRole, err := o.rbac.ClusterRoles().Get(originClusterRole.Name, v1.GetOptions{})
	if err != nil {
		errlist = append(errlist, err)
	}

	// compare the results if there have been no errors so far
	if len(errlist) == 0 {
		// if they are not equal, something has gone wrong and the two objects are not in sync
		if authorizationsync.PrepareForUpdateClusterRole(convertedClusterRole, rbacClusterRole) {
			errlist = append(errlist, errOutOfSync)
		}
	}

	return errlist
}

func (o *MigrateAuthorizationOptions) checkRole(originRole *authorizationapi.Role) []error {
	var errlist []error

	// convert the origin role to a rbac role
	convertedRole, err := authorizationsync.ConvertToRBACRole(originRole)
	if err != nil {
		errlist = append(errlist, err)
	}

	// try to get the equivalent rbac role from the api
	rbacRole, err := o.rbac.Roles(originRole.Namespace).Get(originRole.Name, v1.GetOptions{})
	if err != nil {
		errlist = append(errlist, err)
	}

	// compare the results if there have been no errors so far
	if len(errlist) == 0 {
		// if they are not equal, something has gone wrong and the two objects are not in sync
		if authorizationsync.PrepareForUpdateRole(convertedRole, rbacRole) {
			errlist = append(errlist, errOutOfSync)
		}
	}

	return errlist
}

func (o *MigrateAuthorizationOptions) checkClusterRoleBinding(originRoleBinding *authorizationapi.ClusterRoleBinding) []error {
	var errlist []error

	// convert the origin role binding to a rbac role binding
	convertedRoleBinding, err := authorizationsync.ConvertToRBACClusterRoleBinding(originRoleBinding)
	if err != nil {
		errlist = append(errlist, err)
	}

	// try to get the equivalent rbac role binding from the api
	rbacRoleBinding, err := o.rbac.ClusterRoleBindings().Get(originRoleBinding.Name, v1.GetOptions{})
	if err != nil {
		errlist = append(errlist, err)
	}

	// compare the results if there have been no errors so far
	if len(errlist) == 0 {
		// if they are not equal, something has gone wrong and the two objects are not in sync
		if authorizationsync.PrepareForUpdateClusterRoleBinding(convertedRoleBinding, rbacRoleBinding) {
			errlist = append(errlist, errOutOfSync)
		}
	}

	return errlist
}

func (o *MigrateAuthorizationOptions) checkRoleBinding(originRoleBinding *authorizationapi.RoleBinding) []error {
	var errlist []error

	// convert the origin role binding to a rbac role binding
	convertedRoleBinding, err := authorizationsync.ConvertToRBACRoleBinding(originRoleBinding)
	if err != nil {
		errlist = append(errlist, err)
	}

	// try to get the equivalent rbac role binding from the api
	rbacRoleBinding, err := o.rbac.RoleBindings(originRoleBinding.Namespace).Get(originRoleBinding.Name, v1.GetOptions{})
	if err != nil {
		errlist = append(errlist, err)
	}

	// compare the results if there have been no errors so far
	if len(errlist) == 0 {
		// if they are not equal, something has gone wrong and the two objects are not in sync
		if authorizationsync.PrepareForUpdateRoleBinding(convertedRoleBinding, rbacRoleBinding) {
			errlist = append(errlist, errOutOfSync)
		}
	}

	return errlist
}
