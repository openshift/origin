package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/registry/clusternetwork"
	"github.com/openshift/origin/pkg/user/registry/user"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for sdn against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against subnets
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		Copier:            kapi.Scheme,
		NewFunc:           func() runtime.Object { return &api.ClusterNetwork{} },
		NewListFunc:       func() runtime.Object { return &api.ClusterNetworkList{} },
		PredicateFunc:     clusternetwork.Matcher,
		QualifiedResource: api.Resource("clusternetworks"),

		CreateStrategy: clusternetwork.Strategy,
		UpdateStrategy: clusternetwork.Strategy,
		DeleteStrategy: clusternetwork.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: user.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
