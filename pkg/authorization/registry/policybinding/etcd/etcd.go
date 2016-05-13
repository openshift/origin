package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/policybinding"
)

const PolicyBindingPath = "/authorization/local/policybindings"

type REST struct {
	*registry.Store
}

// NewStorage returns a RESTStorage object that will work against nodes.
func NewStorage(opts generic.RESTOptions) *REST {
	newListFunc := func() runtime.Object { return &authorizationapi.PolicyBindingList{} }

	storageInterface := opts.Decorator(opts.Storage, 100, &authorizationapi.PolicyBindingList{}, PolicyBindingPath, policybinding.Strategy, newListFunc)

	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &authorizationapi.PolicyBinding{} },
		NewListFunc:       newListFunc,
		QualifiedResource: authorizationapi.Resource("policybinding"),
		KeyRootFunc: func(ctx kapi.Context) string {
			return registry.NamespaceKeyRootFunc(ctx, PolicyBindingPath)
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return registry.NamespaceKeyFunc(ctx, PolicyBindingPath, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*authorizationapi.PolicyBinding).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return policybinding.Matcher(label, field)
		},

		CreateStrategy: policybinding.Strategy,
		UpdateStrategy: policybinding.Strategy,

		Storage: storageInterface,
	}

	return &REST{store}
}
