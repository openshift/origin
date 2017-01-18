package etcd

import (
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/rolebindingrestriction"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against nodes.
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &authorizationapi.RoleBindingRestriction{} },
		NewListFunc:       func() runtime.Object { return &authorizationapi.RoleBindingRestrictionList{} },
		QualifiedResource: authorizationapi.Resource("rolebindingrestrictions"),
		PredicateFunc:     rolebindingrestriction.Matcher,

		CreateStrategy: rolebindingrestriction.Strategy,
		UpdateStrategy: rolebindingrestriction.Strategy,
		DeleteStrategy: rolebindingrestriction.Strategy,
	}

	// TODO this will be uncommented after 1.6 rebase:
	// options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: rolebindingrestriction.GetAttrs}
	// if err := store.CompleteWithOptions(options); err != nil {
	if err := restoptions.ApplyOptions(optsGetter, store, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}

	return &REST{Store: store}, nil
}
