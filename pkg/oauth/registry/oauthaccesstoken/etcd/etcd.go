package etcd

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/util/hash"
	"github.com/openshift/origin/pkg/util/observe"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for access tokens against etcd
type REST struct {
	*registry.Store

	hasher hash.HashOptions
}

// NewREST returns a RESTStorage object that will work against access tokens
func NewREST(optsGetter restoptions.Getter, hasher hash.HashOptions, clientGetter oauthclient.Getter, backends ...storage.Interface) (*REST, error) {
	qualifiedResource := api.Resource("oauthaccesstokens")
	opts, err := optsGetter.GetRESTOptions(qualifiedResource)
	if err != nil {
		return nil, err
	}

	strategy := oauthaccesstoken.NewStrategy(clientGetter)
	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.OAuthAccessToken{} },
		NewListFunc: func() runtime.Object { return &api.OAuthAccessTokenList{} },
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.OAuthAccessToken).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
			return oauthaccesstoken.Matcher(label, field)
		},
		KeyRootFunc: func(ctx kapi.Context) string {
			return opts.ResourcePrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			if claims, err := api.ClaimsFromToken(name); err == nil {
				return opts.ResourcePrefix + "/" + claims.UserHash + "/" + claims.SecretHash, nil
			}
			return registry.NoNamespaceKeyFunc(ctx, opts.ResourcePrefix, name)
		},
		TTLFunc: func(obj runtime.Object, existing uint64, update bool) (uint64, error) {
			token := obj.(*api.OAuthAccessToken)
			expires := uint64(token.ExpiresIn)
			return expires, nil
		},
		QualifiedResource: api.Resource("oauthaccesstokens"),

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, false, storage.NoTriggerPublisher); err != nil {
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

	return &REST{store, hasher}, nil
}

// Create inserts a new item according to the unique key from the object.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	token := obj.(*api.OAuthAccessToken)

	// Clear server-set fields
	token.Token = ""
	token.Salt = ""
	token.SaltedHash = ""

	if len(token.Name) > 0 {
		// A legacy client already specified a name, so honor it
		createdObj, err := r.Store.Create(ctx, obj)
		if err != nil {
			return nil, err
		}
		// Decorate the result with the generated token
		createdToken := createdObj.(*api.OAuthAccessToken)
		createdToken.Token = createdToken.Name
		return createdToken, err
	}

	// Generate a secret
	secret, err := r.hasher.Rand(32)
	if err != nil {
		return nil, kerrors.NewInternalError(err)
	}

	// Turn it into a bearer token
	bearerToken := ""
	if r.hasher.HashOnWrite() {
		tokenName, generatedBearerToken, err := api.TokenNameFromUserAndSecret(token.UserName, secret)
		if err != nil {
			return nil, kerrors.NewInternalError(err)
		}

		salt, err := r.hasher.Rand(32)
		if err != nil {
			return nil, kerrors.NewInternalError(err)
		}
		saltedHash := r.hasher.SaltedHash(secret, salt)

		token.Name = tokenName
		token.Salt = salt
		token.SaltedHash = saltedHash
		bearerToken = generatedBearerToken
	} else {
		token.Name = secret
		bearerToken = secret
	}

	createdObj, err := r.Store.Create(ctx, obj)
	if err != nil {
		return nil, err
	}

	// Decorate the result with the generated token
	createdToken := createdObj.(*api.OAuthAccessToken)
	createdToken.Token = bearerToken
	return createdToken, err
}
