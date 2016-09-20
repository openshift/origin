package etcd

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/policybinding"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type REST struct {
	*registry.Store
}

// NewStorage returns a RESTStorage object that will work against nodes.
func NewStorage(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &authorizationapi.PolicyBinding{} },
		NewListFunc:       func() runtime.Object { return &authorizationapi.PolicyBindingList{} },
		QualifiedResource: authorizationapi.Resource("policybindings"),
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*authorizationapi.PolicyBinding).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
			return policybinding.Matcher(label, field)
		},

		CreateStrategy: policybinding.Strategy,
		UpdateStrategy: policybinding.Strategy,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, true, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
