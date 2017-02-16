package rolebindingrestriction

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type REST struct {
	*registry.Store
}

// NewStorage returns a RESTStorage object that will work against nodes.
func NewStorage(optsGetter restoptions.Getter) (*REST, error) {
	store, err := makeStore(optsGetter)
	if err != nil {
		return nil, err
	}

	return &REST{Store: store}, nil
}

func makeStore(optsGetter restoptions.Getter) (*registry.Store, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &authorizationapi.RoleBindingRestriction{} },
		NewListFunc:       func() runtime.Object { return &authorizationapi.RoleBindingRestrictionList{} },
		QualifiedResource: authorizationapi.Resource("rolebindingrestrictions"),
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*authorizationapi.RoleBindingRestriction).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
			return Matcher(label, field)
		},

		CreateStrategy:      Strategy,
		UpdateStrategy:      Strategy,
		DeleteStrategy:      Strategy,
		ReturnDeletedObject: false,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, true, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}

	return store, nil
}
