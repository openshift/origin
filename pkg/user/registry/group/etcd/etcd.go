package etcd

import (
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/registry/group"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// REST implements a RESTStorage for groups against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against groups
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.Group{} },
		NewListFunc:       func() runtime.Object { return &api.GroupList{} },
		PredicateFunc:     group.Matcher,
		QualifiedResource: api.Resource("groups"),

		CreateStrategy: group.Strategy,
		UpdateStrategy: group.Strategy,
	}

	// TODO this will be uncommented after 1.6 rebase:
	// options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: group.GetAttrs}
	// if err := store.CompleteWithOptions(options); err != nil {
	if err := restoptions.ApplyOptions(optsGetter, store, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
