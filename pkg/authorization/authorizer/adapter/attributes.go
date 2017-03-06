package adapter

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kauthorizer "k8s.io/kubernetes/pkg/auth/authorizer"
	"k8s.io/kubernetes/pkg/auth/user"

	oauthorizer "github.com/openshift/origin/pkg/authorization/authorizer"
)

var _ = kauthorizer.Attributes(AdapterAttributes{})

// AdapterAttributes satisfies k8s authorizer.Attributes interfaces
type AdapterAttributes struct {
	namespace               string
	user                    user.Info
	authorizationAttributes oauthorizer.Action
}

// OriginAuthorizerAttributes adapts Kubernetes authorization attributes to Origin authorization attributes
// Note that some info (like resourceName, apiVersion, apiGroup) is not available from the Kubernetes attributes
func OriginAuthorizerAttributes(kattrs kauthorizer.Attributes) (kapi.Context, oauthorizer.Action) {
	// Build a context to hold the namespace and user info
	ctx := kapi.NewContext()
	ctx = kapi.WithNamespace(ctx, kattrs.GetNamespace())
	ctx = kapi.WithUser(ctx, kattrs.GetUser())

	// If we recognize the type, use the embedded type.  Do NOT use it directly, because not all things that quack are ducks.
	if castAdapterAttributes, ok := kattrs.(AdapterAttributes); ok {
		return ctx, castAdapterAttributes.authorizationAttributes
	}

	// Otherwise build what we can
	oattrs := &oauthorizer.DefaultAuthorizationAttributes{
		Verb:         kattrs.GetVerb(),
		APIGroup:     kattrs.GetAPIGroup(),
		APIVersion:   kattrs.GetAPIVersion(),
		Resource:     kattrs.GetResource(),
		Subresource:  kattrs.GetSubresource(),
		ResourceName: kattrs.GetName(),

		NonResourceURL: kattrs.IsResourceRequest() == false,
		URL:            kattrs.GetPath(),
	}

	return ctx, oattrs
}

// KubernetesAuthorizerAttributes adapts Origin authorization attributes to Kubernetes authorization attributes
// The returned attributes can be passed to OriginAuthorizerAttributes to access extra information from the Origin attributes interface
func KubernetesAuthorizerAttributes(namespace string, user user.Info, oattrs oauthorizer.Action) kauthorizer.Attributes {
	return AdapterAttributes{
		namespace: namespace,
		user:      user,
		authorizationAttributes: oattrs,
	}
}

func (a AdapterAttributes) GetVerb() string {
	return a.authorizationAttributes.GetVerb()
}

func (a AdapterAttributes) GetAPIGroup() string {
	return a.authorizationAttributes.GetAPIGroup()
}

func (a AdapterAttributes) GetAPIVersion() string {
	return a.authorizationAttributes.GetAPIVersion()
}

// GetNamespace satisfies the kubernetes authorizer.Attributes interface
// origin gets this value from the request context
func (a AdapterAttributes) GetNamespace() string {
	return a.namespace
}

func (a AdapterAttributes) GetName() string {
	return a.authorizationAttributes.GetResourceName()
}

func (a AdapterAttributes) GetSubresource() string {
	return a.authorizationAttributes.GetSubresource()
}

func (a AdapterAttributes) GetResource() string {
	return a.authorizationAttributes.GetResource()
}

// GetUserName satisfies the kubernetes authorizer.Attributes interface
// origin gets this value from the request context
func (a AdapterAttributes) GetUser() user.Info {
	return a.user
}

// IsReadOnly satisfies the kubernetes authorizer.Attributes interface based on the verb
func (a AdapterAttributes) IsReadOnly() bool {
	v := a.GetVerb()
	return v == "get" || v == "list" || v == "watch"
}

// IsResourceRequest satisfies the kubernetes authorizer.Attributes interface
func (a AdapterAttributes) IsResourceRequest() bool {
	return !a.authorizationAttributes.IsNonResourceURL()
}

// GetPath satisfies the kubernetes authorizer.Attributes interface
func (a AdapterAttributes) GetPath() string {
	return a.authorizationAttributes.GetURL()
}
