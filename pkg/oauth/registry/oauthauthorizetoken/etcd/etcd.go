package etcd

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/util"
	"github.com/openshift/origin/pkg/util/observe"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for authorize tokens against etcd
type REST struct {
	*registry.Store
}

const EtcdPrefix = "/oauth/authorizetokens"

// NewREST returns a RESTStorage object that will work against authorize tokens
func NewREST(optsGetter restoptions.Getter, clientGetter oauthclient.Getter, backends ...storage.Interface) (*REST, error) {
	strategy := oauthauthorizetoken.NewStrategy(clientGetter)
	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.OAuthAuthorizeToken{} },
		NewListFunc: func() runtime.Object { return &api.OAuthAuthorizeTokenList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return EtcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return util.NoNamespaceKeyFunc(ctx, EtcdPrefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.OAuthAuthorizeToken).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return oauthauthorizetoken.Matcher(label, field)
		},
		TTLFunc: func(obj runtime.Object, existing uint64, update bool) (uint64, error) {
			token := obj.(*api.OAuthAuthorizeToken)
			expires := uint64(token.ExpiresIn)
			return expires, nil
		},
		QualifiedResource: api.Resource("oauthauthorizetokens"),

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, EtcdPrefix); err != nil {
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
			return observer.ObserveResourceVersion(obj.(*api.OAuthAuthorizeToken).ResourceVersion, 5*time.Second)
		}
	}

	return &REST{store}, nil
}
