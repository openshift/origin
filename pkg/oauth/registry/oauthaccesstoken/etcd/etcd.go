package etcd

import (
	"time"

	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/util/observe"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for access tokens against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against access tokens
func NewREST(optsGetter restoptions.Getter, clientGetter oauthclient.Getter, backends ...storage.Interface) (*REST, error) {
	strategy := oauthaccesstoken.NewStrategy(clientGetter)
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.OAuthAccessToken{} },
		NewListFunc:       func() runtime.Object { return &api.OAuthAccessTokenList{} },
		PredicateFunc:     oauthaccesstoken.Matcher,
		QualifiedResource: api.Resource("oauthaccesstokens"),

		TTLFunc: func(obj runtime.Object, existing uint64, update bool) (uint64, error) {
			token := obj.(*api.OAuthAccessToken)
			expires := uint64(token.ExpiresIn)
			return expires, nil
		},

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
	}

	// TODO this will be uncommented after 1.6 rebase:
	// options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: oauthaccesstoken.GetAttrs}
	// if err := store.CompleteWithOptions(options); err != nil {
	if err := restoptions.ApplyOptions(optsGetter, store, storage.NoTriggerPublisher); err != nil {
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
			return observer.ObserveResourceVersion(obj.(*api.OAuthAccessToken).ResourceVersion, 5*time.Second)
		}
	}

	return &REST{store}, nil
}
