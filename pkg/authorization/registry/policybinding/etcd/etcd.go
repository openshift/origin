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
	"github.com/openshift/origin/pkg/util/restoptions"
)

const PolicyBindingPath = "/authorization/local/policybindings"

type REST struct {
	*registry.Store
}

// NewStorage returns a RESTStorage object that will work against nodes.
func NewStorage(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &authorizationapi.PolicyBinding{} },
		NewListFunc:       func() runtime.Object { return &authorizationapi.PolicyBindingList{} },
		QualifiedResource: authorizationapi.Resource("policybindings"),
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
	}

	if err := restoptions.ApplyOptions(optsGetter, store, PolicyBindingPath); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
