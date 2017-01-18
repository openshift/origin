package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/quota/api"
	"github.com/openshift/origin/pkg/quota/registry/clusterresourcequota"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against ClusterResourceQuota objects.
func NewREST(optsGetter restoptions.Getter) (*REST, *StatusREST, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.ClusterResourceQuota{} },
		NewListFunc:       func() runtime.Object { return &api.ClusterResourceQuotaList{} },
		PredicateFunc:     clusterresourcequota.Matcher,
		QualifiedResource: api.Resource("clusterresourcequotas"),

		CreateStrategy: clusterresourcequota.Strategy,
		UpdateStrategy: clusterresourcequota.Strategy,
		DeleteStrategy: clusterresourcequota.Strategy,
	}

	// TODO this will be uncommented after 1.6 rebase:
	// options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: clusterresourcequota.GetAttrs}
	// if err := store.CompleteWithOptions(options); err != nil {
	if err := restoptions.ApplyOptions(optsGetter, store, storage.NoTriggerPublisher); err != nil {
		return nil, nil, err
	}

	statusStore := *store
	statusStore.CreateStrategy = nil
	statusStore.DeleteStrategy = nil
	statusStore.UpdateStrategy = clusterresourcequota.StatusStrategy

	return &REST{store}, &StatusREST{store: &statusStore}, nil
}

// StatusREST implements the REST endpoint for changing the status of a resourcequota.
type StatusREST struct {
	store *registry.Store
}

// StatusREST implements Patcher
var _ = rest.Patcher(&StatusREST{})

func (r *StatusREST) New() runtime.Object {
	return &api.ClusterResourceQuota{}
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	return r.store.Get(ctx, name)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx kapi.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo)
}
