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
	"github.com/openshift/origin/pkg/util/restoptions"
)

// REST implements a RESTStorage for identites against etcd
type REST struct {
	registry.Store
}

const EtcdPrefix = "/useridentities"

// NewREST returns a RESTStorage object that will work against identites
func NewREST(optsGetter restoptions.Getter) (*REST, error) {

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

		CreateStrategy: identity.Strategy,
		UpdateStrategy: identity.Strategy,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, EtcdPrefix); err != nil {
		return nil, err
	}

	return &REST{*store}, nil
}
