package etcd

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	kapi "k8s.io/kubernetes/pkg/api"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	"github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/util/observe"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for access tokens against etcd
type REST struct {
	*registry.Store
}

var _ rest.StandardStorage = &REST{}

// NewREST returns a RESTStorage object that will work against access tokens
func NewREST(optsGetter restoptions.Getter, clientGetter oauthclient.Getter, backends ...storage.Interface) (*REST, error) {
	strategy := oauthaccesstoken.NewStrategy(clientGetter)
	store := &registry.Store{
		Copier:                   kapi.Scheme,
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

	if len(backends) > 0 {
		// Build identical stores that talk to a single etcd, so we can verify the token is distributed after creation
		watchers := []rest.Watcher{}
		for i := range backends {
			watcher := *store
			watcher.Storage = backends[i]
			watchers = append(watchers, &watcher)
		}
		// Observe the cluster for the particular resource version, requiring at least one backend to succeed
		observer := observe.NewClusterObserver(store.Storage.Versioner(), watchers, 1)
		// After creation, wait for the new token to propagate
		store.AfterCreate = func(obj runtime.Object) error {
			return observer.ObserveResourceVersion(obj.(*oauthapi.OAuthAccessToken).ResourceVersion, 5*time.Second)
		}
	}

	return &REST{store}, nil
}
