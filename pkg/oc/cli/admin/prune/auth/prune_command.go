package auth

import (
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"

	authv1client "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	oauthv1client "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	securityv1client "github.com/openshift/client-go/security/clientset/versioned/typed/security/v1"
	userv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
)

// PruneRolesOptions holds all the required options for pruning roles.
type PruneAuthOptions struct {
	FilenameOptions resource.FilenameOptions
	Selector        string
	All             bool

	Builder                  *resource.Builder
	RoleBindingClient        rbacv1client.RoleBindingsGetter
	ClusterRoleBindingClient rbacv1client.ClusterRoleBindingsGetter

	UserClient          userv1client.UserV1Interface
	AuthorizationClient authv1client.AuthorizationV1Interface
	OAuthClient         oauthv1client.OauthV1Interface
	SecurityClient      securityv1client.SecurityV1Interface

	genericclioptions.IOStreams
}

func NewPruneAuthOptions(streams genericclioptions.IOStreams) *PruneAuthOptions {
	return &PruneAuthOptions{
		IOStreams: streams,
	}
}

// NewCmdPruneRoles implements the OpenShift cli prune roles command.
func NewCmdPruneAuth(f kcmdutil.Factory, name string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewPruneAuthOptions(streams)
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

	clientConfig, err := f.ToRESTConfig()
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
	o.UserClient, err = userv1client.NewForConfig(clientConfig)
	if err != nil {
		return nil
	}
	o.AuthorizationClient, err = authv1client.NewForConfig(clientConfig)
	if err != nil {
		return nil
	}
	o.OAuthClient, err = oauthv1client.NewForConfig(clientConfig)
	if err != nil {
		return nil
	}
	o.SecurityClient, err = securityv1client.NewForConfig(clientConfig)
	if err != nil {
		return nil
	}

	cmdNamespace, enforceNamespace, err := f.ToRawKubeConfigLoader().Namespace()
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
			reapForUser(o.UserClient, o.AuthorizationClient, o.OAuthClient, o.SecurityClient.SecurityContextConstraints(), info.Name, o.Out)

		case isGroup(info.Mapping):
			reapForGroup(o.AuthorizationClient, o.SecurityClient.SecurityContextConstraints(), info.Name, o.Out)
		}

		return nil
	})

	return err
}

func isRole(mapping *meta.RESTMapping) bool {
	if mapping.Resource.Group != "rbac.authorization.k8s.io" && mapping.Resource.Group != "authorization.openshift.io" {
		return false
	}
	if mapping.Resource.Resource != "roles" {
		return false
	}
	return true
}

func isClusterRole(mapping *meta.RESTMapping) bool {
	if mapping.Resource.Group != "rbac.authorization.k8s.io" && mapping.Resource.Group != "authorization.openshift.io" {
		return false
	}
	if mapping.Resource.Resource != "clusterroles" {
		return false
	}
	return true
}

func isUser(mapping *meta.RESTMapping) bool {
	if mapping.Resource.Group == "user.openshift.io" && mapping.Resource.Resource == "users" {
		return true
	}
	return false
}

func isGroup(mapping *meta.RESTMapping) bool {
	if mapping.Resource.Group == "user.openshift.io" && mapping.Resource.Resource == "groups" {
		return true
	}
	return false
}
