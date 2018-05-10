package authprune

import (
	"io"

	authclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset"
	securitytypedclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"

	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
)

// PruneRolesOptions holds all the required options for pruning roles.
type PruneAuthOptions struct {
	FilenameOptions resource.FilenameOptions
	Selector        string
	All             bool

	Builder                  *resource.Builder
	RoleBindingClient        rbacv1client.RoleBindingsGetter
	ClusterRoleBindingClient rbacv1client.ClusterRoleBindingsGetter

	// TODO switch these to external clients
	UserInternalClient          userclient.Interface
	AuthorizationInternalClient authclient.Interface
	OAuthInternalClient         oauthclient.Interface
	SCCClient                   securitytypedclient.SecurityInterface

	Out io.Writer
}

// NewCmdPruneRoles implements the OpenShift cli prune roles command.
func NewCmdPruneAuth(f kcmdutil.Factory, name string, out io.Writer) *cobra.Command {
	o := &PruneAuthOptions{
		Out: out,
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Removes references to the specified roles, clusterroles, users, and groups.",
		Long:  "Removes references to the specified roles, clusterroles, users, and groups.  Other types are ignored",

		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.RunPrune())
		},
	}

	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, "containing the resource to delete.")

	cmd.Flags().StringVarP(&o.Selector, "selector", "l", "", "Selector (label query) to filter on.")
	cmd.Flags().BoolVar(&o.All, "all", o.All, "Prune all roles in the namespace.")

	return cmd
}

func (o *PruneAuthOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return nil
	}
	o.RoleBindingClient, err = rbacv1client.NewForConfig(clientConfig)
	if err != nil {
		return nil
	}
	o.ClusterRoleBindingClient, err = rbacv1client.NewForConfig(clientConfig)
	if err != nil {
		return nil
	}
	o.UserInternalClient, err = userclient.NewForConfig(clientConfig)
	if err != nil {
		return nil
	}
	o.AuthorizationInternalClient, err = authclient.NewForConfig(clientConfig)
	if err != nil {
		return nil
	}
	o.OAuthInternalClient, err = oauthclient.NewForConfig(clientConfig)
	if err != nil {
		return nil
	}
	o.SCCClient, err = securitytypedclient.NewForConfig(clientConfig)
	if err != nil {
		return nil
	}

	cmdNamespace, enforceNamespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.Builder = f.NewBuilder().
		Unstructured().
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(enforceNamespace, &o.FilenameOptions).
		LabelSelectorParam(o.Selector).
		SelectAllParam(o.All).
		ResourceTypeOrNameArgs(false, args...).
		RequireObject(false).
		Flatten()

	return nil
}

func (o *PruneAuthOptions) RunPrune() error {
	r := o.Builder.Do()
	if r.Err() != nil {
		return r.Err()
	}

	// this is weird, but we do this here for easy compatibility with existing reapers.  This command doesn't make sense
	// without a server connection.  Still.  Don't do this at home.
	err := r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		switch {
		case isRole(info.Mapping):
			reapForRole(o.RoleBindingClient, info.Namespace, info.Name, o.Out)

		case isClusterRole(info.Mapping):
			reapForClusterRole(o.ClusterRoleBindingClient, o.RoleBindingClient, info.Namespace, info.Name, o.Out)

		case isUser(info.Mapping):
			reapForUser(o.UserInternalClient, o.AuthorizationInternalClient, o.OAuthInternalClient, o.SCCClient.SecurityContextConstraints(), info.Name, o.Out)

		case isGroup(info.Mapping):
			reapForGroup(o.AuthorizationInternalClient, o.SCCClient.SecurityContextConstraints(), info.Name, o.Out)

		}

		return nil
	})

	return err
}

func isRole(mapping *meta.RESTMapping) bool {
	if mapping.GroupVersionKind.Group != "rbac.authorization.k8s.io" && mapping.GroupVersionKind.Group != "authorization.openshift.io" {
		return false
	}
	if mapping.Resource != "roles" {
		return false
	}
	return true
}

func isClusterRole(mapping *meta.RESTMapping) bool {
	if mapping.GroupVersionKind.Group != "rbac.authorization.k8s.io" && mapping.GroupVersionKind.Group != "authorization.openshift.io" {
		return false
	}
	if mapping.Resource != "clusterroles" {
		return false
	}
	return true
}

func isUser(mapping *meta.RESTMapping) bool {
	if mapping.GroupVersionKind.Group == "user.openshift.io" && mapping.Resource == "users" {
		return true
	}
	return false
}

func isGroup(mapping *meta.RESTMapping) bool {
	if mapping.GroupVersionKind.Group == "user.openshift.io" && mapping.Resource == "groups" {
		return true
	}
	return false
}
