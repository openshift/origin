package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/policy"
	"github.com/openshift/origin/pkg/util/restoptions"
)

const PolicyPath = "/authorization/local/policies"

type REST struct {
	*registry.Store
}

// NewStorage returns a RESTStorage object that will work against nodes.
func NewStorage(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &authorizationapi.Policy{} },
		NewListFunc:       func() runtime.Object { return &authorizationapi.PolicyList{} },
		QualifiedResource: authorizationapi.Resource("policies"),
		KeyRootFunc: func(ctx kapi.Context) string {
			return registry.NamespaceKeyRootFunc(ctx, PolicyPath)
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return registry.NamespaceKeyFunc(ctx, PolicyPath, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*authorizationapi.Policy).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return policy.Matcher(label, field)
		},

		CreateStrategy: policy.Strategy,
		UpdateStrategy: policy.Strategy,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, PolicyPath); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
