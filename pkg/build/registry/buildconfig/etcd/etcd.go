package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/buildconfig"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// REST implements the REST endpoint for a BuildConfig
type REST struct {
	*registry.Store
}

// StatusREST implements the REST endpoint for the BuildConfig status
type StatusREST struct {
	store *registry.Store
}

// NewStorage returns a RESTStorage object that will work against nodes.
func NewREST(optsGetter restoptions.Getter) (*REST, *StatusREST, error) {
	prefix := "/buildconfigs"

	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.BuildConfig{} },
		NewListFunc:       func() runtime.Object { return &api.BuildConfigList{} },
		QualifiedResource: api.Resource("buildconfigs"),
		KeyRootFunc: func(ctx kapi.Context) string {
			return registry.NamespaceKeyRootFunc(ctx, prefix)
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return registry.NamespaceKeyFunc(ctx, prefix, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.BuildConfig).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return buildconfig.Matcher(label, field)
		},

		CreateStrategy:      buildconfig.Strategy,
		UpdateStrategy:      buildconfig.Strategy,
		DeleteStrategy:      buildconfig.Strategy,
		ReturnDeletedObject: false,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, prefix); err != nil {
		return nil, nil, err
	}

	statusStore := *store
	statusStore.UpdateStrategy = buildconfig.StatusStrategy

	return &REST{store}, &StatusREST{store: &statusStore}, nil
}

// New creates a new pod resource
func (r *StatusREST) New() runtime.Object {
	return &api.BuildConfig{}
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	return r.store.Get(ctx, name)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx kapi.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo)
}
