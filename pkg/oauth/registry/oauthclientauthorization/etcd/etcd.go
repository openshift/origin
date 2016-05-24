package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization"
	"github.com/openshift/origin/pkg/util"
)

// rest implements a RESTStorage for oauth client authorizations against etcd
type REST struct {
	registry.Store
}

const EtcdPrefix = "/oauth/clientauthorizations"

// NewREST returns a RESTStorage object that will work against oauth clients

func NewREST(opts generic.RESTOptions, clientGetter oauthclient.Getter) *REST {
	newListFunc := func() runtime.Object { return &api.OAuthClientAuthorizationList{} }
	storageInterface := opts.Decorator(opts.Storage, 100, &api.OAuthClientAuthorizationList{}, EtcdPrefix, oauthclientauthorization.NewStrategy(clientGetter), newListFunc)

	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.OAuthClientAuthorization{} },
		NewListFunc: newListFunc,
		KeyRootFunc: func(ctx kapi.Context) string {
			return EtcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return util.NoNamespaceKeyFunc(ctx, EtcdPrefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.OAuthClientAuthorization).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return oauthclientauthorization.Matcher(label, field)
		},
		QualifiedResource: api.Resource("oauthclientauthorizations"),

		Storage: storageInterface,
	}

	store.CreateStrategy = oauthclientauthorization.NewStrategy(clientGetter)
	store.UpdateStrategy = oauthclientauthorization.NewStrategy(clientGetter)

	return &REST{*store}
}
