package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/registry/rolebindingrestriction"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type REST struct {
	*registry.Store
}

var _ rest.StandardStorage = &REST{}

// NewREST returns a RESTStorage object that will work against nodes.
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &authorizationapi.RoleBindingRestriction{} },
		NewListFunc:              func() runtime.Object { return &authorizationapi.RoleBindingRestrictionList{} },
		DefaultQualifiedResource: authorizationapi.Resource("rolebindingrestrictions"),

		CreateStrategy: rolebindingrestriction.Strategy,
		UpdateStrategy: rolebindingrestriction.Strategy,
		DeleteStrategy: rolebindingrestriction.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{Store: store}, nil
}
