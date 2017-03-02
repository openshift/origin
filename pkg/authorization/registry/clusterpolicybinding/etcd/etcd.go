package etcd

import (
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against ClusterPolicyBinding.
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.ClusterPolicyBinding{} },
		NewListFunc:       func() runtime.Object { return &api.ClusterPolicyBindingList{} },
		PredicateFunc:     clusterpolicybinding.Matcher,
		QualifiedResource: api.Resource("clusterpolicybindings"),

		CreateStrategy: clusterpolicybinding.Strategy,
		UpdateStrategy: clusterpolicybinding.Strategy,
	}

	// TODO this will be uncommented after 1.6 rebase:
	// options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: clusterpolicybinding.GetAttrs}
	// if err := store.CompleteWithOptions(options); err != nil {
	if err := restoptions.ApplyOptions(optsGetter, store, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
