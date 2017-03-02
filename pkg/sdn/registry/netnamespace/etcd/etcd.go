package etcd

import (
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/registry/netnamespace"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for sdn against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against netnamespaces
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.NetNamespace{} },
		NewListFunc:       func() runtime.Object { return &api.NetNamespaceList{} },
		PredicateFunc:     netnamespace.Matcher,
		QualifiedResource: api.Resource("netnamespaces"),

		CreateStrategy: netnamespace.Strategy,
		UpdateStrategy: netnamespace.Strategy,
	}

	// TODO this will be uncommented after 1.6 rebase:
	// options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: netnamespace.GetAttrs}
	// if err := store.CompleteWithOptions(options); err != nil {
	if err := restoptions.ApplyOptions(optsGetter, store, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
