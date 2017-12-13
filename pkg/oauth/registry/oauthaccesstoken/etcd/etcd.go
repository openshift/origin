package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	"github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for access tokens against etcd
type REST struct {
	*registry.Store
}

var _ rest.StandardStorage = &REST{}

// NewREST returns a RESTStorage object that will work against access tokens
func NewREST(optsGetter restoptions.Getter, clientGetter oauthclient.Getter) (*REST, error) {
	strategy := oauthaccesstoken.NewStrategy(clientGetter)
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &oauthapi.OAuthAccessToken{} },
		NewListFunc:              func() runtime.Object { return &oauthapi.OAuthAccessTokenList{} },
		DefaultQualifiedResource: oauthapi.Resource("oauthaccesstokens"),

		TTLFunc: func(obj runtime.Object, existing uint64, update bool) (uint64, error) {
			token := obj.(*oauthapi.OAuthAccessToken)
			expires := uint64(token.ExpiresIn)
			return expires, nil
		},

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
		DeleteStrategy: strategy,
	}

	options := &generic.StoreOptions{
		RESTOptions: optsGetter,
		AttrFunc:    storage.AttrFunc(storage.DefaultNamespaceScopedAttr).WithFieldMutation(oauthapi.OAuthAccessTokenFieldSelector),
	}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
