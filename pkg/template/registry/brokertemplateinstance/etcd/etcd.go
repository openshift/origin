package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/template/api"
	rest "github.com/openshift/origin/pkg/template/registry/brokertemplateinstance"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// REST implements a RESTStorage for brokertemplateinstances against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against brokertemplateinstances.
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.BrokerTemplateInstance{} },
		NewListFunc:       func() runtime.Object { return &api.BrokerTemplateInstanceList{} },
		PredicateFunc:     brokertemplateinstance.Matcher,
		QualifiedResource: api.Resource("brokertemplateinstances"),

		CreateStrategy: brokertemplateinstance.Strategy,
		UpdateStrategy: brokertemplateinstance.Strategy,
		DeleteStrategy: brokertemplateinstance.Strategy,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
