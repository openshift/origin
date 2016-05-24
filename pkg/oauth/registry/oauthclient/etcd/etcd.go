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
	"github.com/openshift/origin/pkg/util"
)

// rest implements a RESTStorage for oauth clients against etcd
type REST struct {
	registry.Store
}

const EtcdPrefix = "/oauth/clients"

// NewREST returns a RESTStorage object that will work against oauth clients
func NewREST(opts generic.RESTOptions) *REST {
	newListFunc := func() runtime.Object { return &api.OAuthClientList{} }

	storageInterface := opts.Decorator(opts.Storage, 100, &api.OAuthClientList{}, EtcdPrefix, oauthclient.Strategy, newListFunc)

	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.OAuthClient{} },
		NewListFunc: newListFunc,
		KeyRootFunc: func(ctx kapi.Context) string {
			return EtcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return util.NoNamespaceKeyFunc(ctx, EtcdPrefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.OAuthClient).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return oauthclient.Matcher(label, field)
		},
		QualifiedResource: api.Resource("oauthclients"),

		Storage: storageInterface,
	}

	store.CreateStrategy = oauthclient.Strategy
	store.UpdateStrategy = oauthclient.Strategy

	return &REST{*store}
}
