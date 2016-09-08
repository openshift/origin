package clusterresourcequota

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	quotaapi "github.com/openshift/origin/pkg/quota/api"
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

func NewStatusStorage(optsGetter restoptions.Getter) (*StatusREST, error) {
	store, err := makeStore(optsGetter)
	if err != nil {
		return nil, err
	}
	store.CreateStrategy = nil
	store.DeleteStrategy = nil
	store.UpdateStrategy = StatusStrategy

	return &StatusREST{store: store}, nil
}

// StatusREST implements the REST endpoint for changing the status of a resourcequota.
type StatusREST struct {
	store *registry.Store
}

func (r *StatusREST) New() runtime.Object {
	return &quotaapi.ClusterResourceQuota{}
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	return r.store.Get(ctx, name)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx kapi.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo)
}

func makeStore(optsGetter restoptions.Getter) (*registry.Store, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &quotaapi.ClusterResourceQuota{} },
		NewListFunc:       func() runtime.Object { return &quotaapi.ClusterResourceQuotaList{} },
		QualifiedResource: quotaapi.Resource("clusterresourcequotas"),
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*quotaapi.ClusterResourceQuota).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
			return Matcher(label, field)
		},

		CreateStrategy:      Strategy,
		UpdateStrategy:      Strategy,
		DeleteStrategy:      Strategy,
		ReturnDeletedObject: false,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, false, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}

	return store, nil
}
