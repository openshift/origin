package etcd

import (
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for authorize tokens against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against authorize tokens
func NewREST(optsGetter restoptions.Getter, clientGetter oauthclient.Getter) (*REST, error) {
	strategy := oauthauthorizetoken.NewStrategy(clientGetter)
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.OAuthAuthorizeToken{} },
		NewListFunc:       func() runtime.Object { return &api.OAuthAuthorizeTokenList{} },
		PredicateFunc:     oauthauthorizetoken.Matcher,
		QualifiedResource: api.Resource("oauthauthorizetokens"),

		TTLFunc: func(obj runtime.Object, existing uint64, update bool) (uint64, error) {
			token := obj.(*api.OAuthAuthorizeToken)
			expires := uint64(token.ExpiresIn)
			return expires, nil
		},

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
	}

	// TODO this will be uncommented after 1.6 rebase:
	// options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: oauthauthorizetoken.GetAttrs}
	// if err := store.CompleteWithOptions(options); err != nil {
	if err := restoptions.ApplyOptions(optsGetter, store, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}
	return &REST{store}, nil
}
