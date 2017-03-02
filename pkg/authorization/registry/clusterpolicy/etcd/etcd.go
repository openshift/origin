package etcd

import (
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against ClusterPolicy.
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &authorizationapi.ClusterPolicy{} },
		NewListFunc:       func() runtime.Object { return &authorizationapi.ClusterPolicyList{} },
		QualifiedResource: authorizationapi.Resource("clusterpolicies"),
		PredicateFunc:     clusterpolicy.Matcher,

		CreateStrategy: clusterpolicy.Strategy,
		UpdateStrategy: clusterpolicy.Strategy,
	}

	// TODO this will be uncommented after 1.6 rebase:
	// options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: clusterpolicy.GetAttrs}
	// if err := store.CompleteWithOptions(options); err != nil {
	if err := restoptions.ApplyOptions(optsGetter, store, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
