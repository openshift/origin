package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/registry/identity"
	"github.com/openshift/origin/pkg/util"
)

// REST implements a RESTStorage for identites against etcd
type REST struct {
	*registry.Store
}

const EtcdPrefix = "/useridentities"

// NewREST returns a RESTStorage object that will work against identites
func NewREST(opts generic.RESTOptions) *REST {
	newListFunc := func() runtime.Object { return &api.Identity{} }
	storageInterface := opts.Decorator(opts.Storage, 100, &api.IdentityList{}, EtcdPrefix, identity.Strategy, newListFunc)

	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.Identity{} },
		NewListFunc: func() runtime.Object { return &api.IdentityList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return EtcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return util.NoNamespaceKeyFunc(ctx, EtcdPrefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.Identity).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return identity.Matcher(label, field)
		},
		QualifiedResource: api.Resource("identities"),

		Storage: storageInterface,
	}

	store.CreateStrategy = identity.Strategy
	store.UpdateStrategy = identity.Strategy

	return &REST{store}
}
