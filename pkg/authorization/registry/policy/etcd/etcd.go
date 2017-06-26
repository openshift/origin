package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/registry/policy"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against Policy objects.
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		Copier:            kapi.Scheme,
		NewFunc:           func() runtime.Object { return &authorizationapi.Policy{} },
		NewListFunc:       func() runtime.Object { return &authorizationapi.PolicyList{} },
		PredicateFunc:     policy.Matcher,
		QualifiedResource: authorizationapi.Resource("policies"),

		CreateStrategy: policy.Strategy,
		UpdateStrategy: policy.Strategy,
		DeleteStrategy: policy.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: policy.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
