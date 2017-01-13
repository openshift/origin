package etcd

import (
	"fmt"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	kubeerr "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/helpers"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for oauth client authorizations against etcd
type REST struct {
	registry.Store
}

type SelfREST struct {
	helpers.ReadAndDeleteStorage
}

// NewREST returns a RESTStorage object that will work against oauth clients authorizations
func NewREST(optsGetter restoptions.Getter, clientGetter oauthclient.Getter) (*REST, *SelfREST, error) {
	resource, prefix, err := helpers.GetResourceAndPrefix(optsGetter, "oauthclientauthorizations")
	if err != nil {
		return nil, nil, fmt.Errorf("error building RESTOptions for %s store: %v", resource.String(), err)
	}
	strategy := oauthclientauthorization.NewStrategy(clientGetter)

	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.OAuthClientAuthorization{} },
		NewListFunc: func() runtime.Object { return &api.OAuthClientAuthorizationList{} },
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			// Check to see if the name has the new format and thus is stored per user instead of a flat list
			if username, clientname, err := helpers.SplitClientAuthorizationName(name); err == nil {
				return registry.NoNamespaceKeyFunc(ctx, helpers.GetKeyWithUsername(prefix, username), clientname)
			}
			// Fallback to the old location if the name does not meet the new format
			return registry.NoNamespaceKeyFunc(ctx, prefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.OAuthClientAuthorization).Name, nil
		},
		PredicateFunc:     oauthclientauthorization.Matcher,
		QualifiedResource: resource,

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, false, storage.NoTriggerPublisher); err != nil {
		return nil, nil, err
	}

	// Make a copy of the store so that we can mutate it
	selfFilterConverter := getSelfFilterConverter(*store, prefix)

	return &REST{*store}, &SelfREST{selfFilterConverter}, nil
}

func getSelfFilterConverter(selfStore registry.Store, prefix string) helpers.ReadAndDeleteStorage {
	selfStore.QualifiedResource = api.Resource("selfoauthclientauthorizations")
	selfStore.CreateStrategy = nil
	selfStore.UpdateStrategy = nil

	// We cannot override KeyFunc because the cacher does not provide the user in the context
	// The cacher does not use the KeyRootFunc so it is safe to override
	// This makes watches more efficient by making them user specific (but not UID specific)
	selfStore.KeyRootFunc = helpers.NewUserKeyRootFunc(prefix)

	// Enforce that the OAuthClientAuthorization's userUID is the same as the current user's
	selfObjectUIDFilter := helpers.NewUserUIDFilterFunc(selfStore.PredicateFunc)

	return helpers.NewFilterConverter(
		&selfStore,
		toSelfObject,
		selfObjectUIDFilter,
		ToSelfList,
		helpers.UserUIDListFilterFunc, // Enforce that the OAuthClientAuthorizations in the List have a userUID that is the same as the current user's
		selfNamer,
		selfStore.QualifiedResource,
	)
}

// Convert from OAuthClientAuthorization to SelfOAuthClientAuthorization
func toSelfObject(obj runtime.Object) runtime.Object {
	in, ok := obj.(*api.OAuthClientAuthorization)
	if !ok { // Handle cases where we are passed other objects such as during Delete
		return obj
	}
	out := &api.SelfOAuthClientAuthorization{
		ObjectMeta: in.ObjectMeta, // TODO: do we want to be more specific here?
		ClientName: in.ClientName,
		Scopes:     in.Scopes,
	}
	out.Name = in.ClientName // The user sees the name as the ClientName so they do not have to see their own username repeated
	return out
}

// Convert from OAuthClientAuthorizationList to SelfOAuthClientAuthorizationList
// Exported for testing
func ToSelfList(obj runtime.Object) runtime.Object {
	in := obj.(*api.OAuthClientAuthorizationList)
	out := &api.SelfOAuthClientAuthorizationList{}
	out.ResourceVersion = in.ResourceVersion
	if len(in.Items) == 0 {
		return out
	}
	out.Items = make([]api.SelfOAuthClientAuthorization, 0, len(in.Items))
	for _, item := range in.Items {
		out.Items = append(out.Items, *(toSelfObject(&item).(*api.SelfOAuthClientAuthorization)))
	}
	return out
}

// This simulates overriding the KeyFunc
func selfNamer(ctx kapi.Context, name string) (string, error) {
	if strings.Contains(name, helpers.UserSpaceSeparator) { // This makes sure that the KeyFunc cannot be manipulated to leak data
		return "", kubeerr.NewBadRequest("Invalid name: " + name)
	}
	user, ok := kapi.UserFrom(ctx)
	if !ok {
		return "", kubeerr.NewBadRequest("User parameter required.")
	}
	return helpers.MakeClientAuthorizationName(user.GetName(), name), nil
}
