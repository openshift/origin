package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/build"
)

type REST struct {
	*etcdgeneric.Etcd
}

// NewStorage returns a RESTStorage object that will work against Build objects.
func NewREST(s storage.Interface) (*REST, *DetailsREST) {
	prefix := "/builds"

	store := &etcdgeneric.Etcd{
		NewFunc:           func() runtime.Object { return &api.Build{} },
		NewListFunc:       func() runtime.Object { return &api.BuildList{} },
		QualifiedResource: api.Resource("builds"),
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdgeneric.NamespaceKeyRootFunc(ctx, prefix)
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return etcdgeneric.NamespaceKeyFunc(ctx, prefix, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.Build).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return build.Matcher(label, field)
		},
		CreateStrategy:      build.Strategy,
		UpdateStrategy:      build.Strategy,
		DeleteStrategy:      build.Strategy,
		Decorator:           build.Decorator,
		ReturnDeletedObject: false,
		Storage:             s,
	}

	detailsStore := *store
	detailsStore.UpdateStrategy = build.DetailsStrategy

	return &REST{store}, &DetailsREST{&detailsStore}
}

type DetailsREST struct {
	store *etcdgeneric.Etcd
}

// New returns an empty object that can be used with Update after request data has been put into it.
func (r *DetailsREST) New() runtime.Object {
	return r.store.New()
}

// Update finds a resource in the storage and updates it.
func (r *DetailsREST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return r.store.Update(ctx, obj)
}
