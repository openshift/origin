package authorization

import (
	"errors"
	"fmt"
	"io"

	"k8s.io/api/rbac/v1beta1"
	kerrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/flowcontrol"
	rbacinternalversion "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/rbac/internalversion"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/registry/util"
	"github.com/openshift/origin/pkg/oc/admin/migrate"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"

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

	// errOutOfSync is retriable since it could be caused by the controller lagging behind
	errOutOfSync = migrate.ErrRetriable{errors.New("is not in sync with RBAC")}
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
			Confirm:       true, // force our save function to always run (it is read only)
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
		Deprecated: fmt.Sprintf("will not work against 3.7 or later servers"),
	}
	return cmd
}

func (o *MigrateAuthorizationOptions) Complete(name string, f *clientcmd.Factory, c *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("%s takes no positional arguments", name)
	}

	o.ResourceOptions.SaveFn = o.checkParity
	if err := o.ResourceOptions.Complete(f, c); err != nil {
		return err
	}

	discovery, err := f.DiscoveryClient()
	if err != nil {
		return err
	}

	if err := clientcmd.LegacyPolicyResourceGate(discovery); err != nil {
		return err
	}

	config, err := f.ClientConfig()
	if err != nil {
		return err
	}

	// do not rate limit this client because it has to scan all RBAC data across the cluster
	// this is safe because only a cluster admin will have the ability to read these objects
	configShallowCopy := *config
	configShallowCopy.RateLimiter = flowcontrol.NewFakeAlwaysRateLimiter()

	// This command is only compatible with a 3.6 server, which only supported RBAC v1beta1
	// Thus we must force that GV otherwise the client will default to v1
	gv := v1beta1.SchemeGroupVersion
	configShallowCopy.GroupVersion = &gv

	rbac, err := rbacinternalversion.NewForConfig(&configShallowCopy)
	if err != nil {
		return err
	}
	o.rbac = rbac

	return nil
}

func (o MigrateAuthorizationOptions) Validate() error {
	return o.ResourceOptions.Validate()
}

func (o MigrateAuthorizationOptions) Run() error {
	// we lie and say this object has changed so our save function will run
	return o.ResourceOptions.Visitor().Visit(migrate.AlwaysRequiresMigration)
}

// checkParity confirms that Openshift authorization objects are in sync with Kubernetes RBAC
// and returns an error if they are out of sync or if it encounters a conversion error
func (o *MigrateAuthorizationOptions) checkParity(info *resource.Info, _ migrate.Reporter) error {
	var err migrate.TemporaryError

	switch t := info.Object.(type) {
	case *authorizationapi.ClusterRole:
		err = o.checkClusterRole(t)
	case *authorizationapi.Role:
		err = o.checkRole(t)
	case *authorizationapi.ClusterRoleBinding:
		err = o.checkClusterRoleBinding(t)
	case *authorizationapi.RoleBinding:
		err = o.checkRoleBinding(t)
	default:
		// this should never happen unless o.Include or the server is broken
		return fmt.Errorf("impossible type %T for checkParity info=%#v object=%#v", t, info, t)
	}

	// We encountered no error, so this object is in sync.
	if err == nil {
		// we only perform read operations so we return this error to signal that we did not change anything
		return migrate.ErrUnchanged
	}

	// At this point we know that we have some non-nil TemporaryError.
	// If it has the possibility of being transient, we need to sync ourselves with the current state of the object.
	if err.Temporary() {
		// The most likely cause is that an authorization object was deleted after we initially fetched it,
		// and the controller deleted the associated RBAC object, which caused our RBAC GET to fail.
		// We can confirm this by refetching the authorization object.
		refreshErr := info.Get()
		if refreshErr != nil {
			// Our refresh GET for this authorization object failed.
			// The default logic for migration errors is appropriate in this case (refreshErr is most likely a NotFound).
			return migrate.DefaultRetriable(info, refreshErr)
		}
		// We had no refreshErr, but encountered some other possibly transient error.
		// No special handling is required in this case, we just pass it through below.
	}

	// All of the check* funcs return an appropriate TemporaryError based on the failure,
	// so we can pass that through to the default migration logic which will retry as needed.
	return err
}

// handleRBACGetError signals for a retry on NotFound (handles deletion and sync lag)
// and ServerTimeout (handles heavy load against the server).
func handleRBACGetError(err error) migrate.TemporaryError {
	switch {
	case kerrs.IsNotFound(err), kerrs.IsServerTimeout(err):
		return migrate.ErrRetriable{err}
	default:
		return migrate.ErrNotRetriable{err}
	}
}

func (o *MigrateAuthorizationOptions) checkClusterRole(originClusterRole *authorizationapi.ClusterRole) migrate.TemporaryError {
	// convert the origin role to a rbac role
	convertedClusterRole, err := util.ConvertToRBACClusterRole(originClusterRole)
	if err != nil {
		// conversion errors should basically never happen, so we do not attempt to retry on those
		return migrate.ErrNotRetriable{err}
	}

	// try to get the equivalent rbac role from the api
	rbacClusterRole, err := o.rbac.ClusterRoles().Get(originClusterRole.Name, v1.GetOptions{})
	if err != nil {
		// it is possible that the controller has not synced this yet
		return handleRBACGetError(err)
	}

	// if they are not equal, something has gone wrong and the two objects are not in sync
	if util.PrepareForUpdateClusterRole(convertedClusterRole, rbacClusterRole) {
		// we retry on this since it could be caused by the controller lagging behind
		return errOutOfSync
	}

	return nil
}

func (o *MigrateAuthorizationOptions) checkRole(originRole *authorizationapi.Role) migrate.TemporaryError {
	// convert the origin role to a rbac role
	convertedRole, err := util.ConvertToRBACRole(originRole)
	if err != nil {
		// conversion errors should basically never happen, so we do not attempt to retry on those
		return migrate.ErrNotRetriable{err}
	}

	// try to get the equivalent rbac role from the api
	rbacRole, err := o.rbac.Roles(originRole.Namespace).Get(originRole.Name, v1.GetOptions{})
	if err != nil {
		// it is possible that the controller has not synced this yet
		return handleRBACGetError(err)
	}

	// if they are not equal, something has gone wrong and the two objects are not in sync
	if util.PrepareForUpdateRole(convertedRole, rbacRole) {
		// we retry on this since it could be caused by the controller lagging behind
		return errOutOfSync
	}

	return nil
}

func (o *MigrateAuthorizationOptions) checkClusterRoleBinding(originRoleBinding *authorizationapi.ClusterRoleBinding) migrate.TemporaryError {
	// convert the origin role binding to a rbac role binding
	convertedRoleBinding, err := util.ConvertToRBACClusterRoleBinding(originRoleBinding)
	if err != nil {
		// conversion errors should basically never happen, so we do not attempt to retry on those
		return migrate.ErrNotRetriable{err}
	}

	// try to get the equivalent rbac role binding from the api
	rbacRoleBinding, err := o.rbac.ClusterRoleBindings().Get(originRoleBinding.Name, v1.GetOptions{})
	if err != nil {
		// it is possible that the controller has not synced this yet
		return handleRBACGetError(err)
	}

	// if they are not equal, something has gone wrong and the two objects are not in sync
	if util.PrepareForUpdateClusterRoleBinding(convertedRoleBinding, rbacRoleBinding) {
		// we retry on this since it could be caused by the controller lagging behind
		return errOutOfSync
	}

	return nil
}

func (o *MigrateAuthorizationOptions) checkRoleBinding(originRoleBinding *authorizationapi.RoleBinding) migrate.TemporaryError {
	// convert the origin role binding to a rbac role binding
	convertedRoleBinding, err := util.ConvertToRBACRoleBinding(originRoleBinding)
	if err != nil {
		// conversion errors should basically never happen, so we do not attempt to retry on those
		return migrate.ErrNotRetriable{err}
	}

	// try to get the equivalent rbac role binding from the api
	rbacRoleBinding, err := o.rbac.RoleBindings(originRoleBinding.Namespace).Get(originRoleBinding.Name, v1.GetOptions{})
	if err != nil {
		// it is possible that the controller has not synced this yet
		return handleRBACGetError(err)
	}

	// if they are not equal, something has gone wrong and the two objects are not in sync
	if util.PrepareForUpdateRoleBinding(convertedRoleBinding, rbacRoleBinding) {
		// we retry on this since it could be caused by the controller lagging behind
		return errOutOfSync
	}

	return nil
}
