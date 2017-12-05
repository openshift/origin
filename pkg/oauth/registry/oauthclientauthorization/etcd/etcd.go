package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for oauth client authorizations against etcd
type REST struct {
	*registry.Store
}

var _ rest.StandardStorage = &REST{}

// NewREST returns a RESTStorage object that will work against oauth clients
func NewREST(optsGetter restoptions.Getter, clientGetter oauthclient.Getter) (*REST, error) {
	strategy := oauthclientauthorization.NewStrategy(clientGetter)

	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &oauthapi.OAuthClientAuthorization{} },
		NewListFunc:              func() runtime.Object { return &oauthapi.OAuthClientAuthorizationList{} },
		DefaultQualifiedResource: oauthapi.Resource("oauthclientauthorizations"),

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
		DeleteStrategy: strategy,
	}

	options := &generic.StoreOptions{
		RESTOptions: optsGetter,
		AttrFunc:    storage.AttrFunc(storage.DefaultNamespaceScopedAttr).WithFieldMutation(oauthapi.OAuthClientAuthorizationFieldSelector),
	}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
