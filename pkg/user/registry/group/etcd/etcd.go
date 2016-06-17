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
	"github.com/openshift/origin/pkg/util/restoptions"
)

const EtcdPrefix = "/groups"

// REST implements a RESTStorage for groups against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against groups
func NewREST(optsGetter restoptions.Getter) (*REST, error) {

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
	}

	if err := restoptions.ApplyOptions(optsGetter, store, EtcdPrefix); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
