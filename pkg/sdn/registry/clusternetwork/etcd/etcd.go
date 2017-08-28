package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	sdnapi "github.com/openshift/origin/pkg/sdn/apis/network"
	"github.com/openshift/origin/pkg/sdn/registry/clusternetwork"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for sdn against etcd
type REST struct {
	*registry.Store
}

var _ rest.StandardStorage = &REST{}

// NewREST returns a RESTStorage object that will work against subnets
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		Copier:                   kapi.Scheme,
		NewFunc:                  func() runtime.Object { return &sdnapi.ClusterNetwork{} },
		NewListFunc:              func() runtime.Object { return &sdnapi.ClusterNetworkList{} },
		DefaultQualifiedResource: sdnapi.Resource("clusternetworks"),

		CreateStrategy: clusternetwork.Strategy,
		UpdateStrategy: clusternetwork.Strategy,
		DeleteStrategy: clusternetwork.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
