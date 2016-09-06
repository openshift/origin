package etcd

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
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
		NewFunc:     func() runtime.Object { return &api.OAuthAuthorizeToken{} },
		NewListFunc: func() runtime.Object { return &api.OAuthAuthorizeTokenList{} },
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.OAuthAuthorizeToken).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
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

	if err := restoptions.ApplyOptions(optsGetter, store, false, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}
	return &REST{store}, nil
}
