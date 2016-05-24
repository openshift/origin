package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/registry/group"
	"github.com/openshift/origin/pkg/util"
)

const EtcdPrefix = "/groups"

// REST implements a RESTStorage for groups against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against groups
func NewREST(opts generic.RESTOptions) *REST {
	newListFunc := func() runtime.Object { return &api.GroupList{} }
	storageInterface := opts.Decorator(opts.Storage, 100, &api.GroupList{}, EtcdPrefix, group.Strategy, newListFunc)

	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.Group{} },
		NewListFunc: func() runtime.Object { return &api.GroupList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return EtcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return util.NoNamespaceKeyFunc(ctx, EtcdPrefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.Group).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return group.Matcher(label, field)
		},
		QualifiedResource: api.Resource("groups"),

		CreateStrategy: group.Strategy,
		UpdateStrategy: group.Strategy,

		Storage: storageInterface,
	}

	return &REST{store}
}
