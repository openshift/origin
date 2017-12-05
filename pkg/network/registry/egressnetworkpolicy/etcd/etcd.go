package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/registry/egressnetworkpolicy"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for egress network policy against etcd
type REST struct {
	*registry.Store
}

var _ rest.StandardStorage = &REST{}

// NewREST returns a RESTStorage object that will work against egress network policy
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &networkapi.EgressNetworkPolicy{} },
		NewListFunc:              func() runtime.Object { return &networkapi.EgressNetworkPolicyList{} },
		DefaultQualifiedResource: networkapi.Resource("egressnetworkpolicies"),

		CreateStrategy: egressnetworkpolicy.Strategy,
		UpdateStrategy: egressnetworkpolicy.Strategy,
		DeleteStrategy: egressnetworkpolicy.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
