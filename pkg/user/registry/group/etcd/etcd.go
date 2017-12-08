package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
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
		NewFunc:                  func() runtime.Object { return &userapi.Group{} },
		NewListFunc:              func() runtime.Object { return &userapi.GroupList{} },
		DefaultQualifiedResource: userapi.Resource("groups"),

		CreateStrategy: group.Strategy,
		UpdateStrategy: group.Strategy,
		DeleteStrategy: group.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
