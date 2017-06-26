package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	kapi "k8s.io/kubernetes/pkg/api"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
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
		Copier:            kapi.Scheme,
		NewFunc:           func() runtime.Object { return &templateapi.BrokerTemplateInstance{} },
		NewListFunc:       func() runtime.Object { return &templateapi.BrokerTemplateInstanceList{} },
		PredicateFunc:     rest.Matcher,
		QualifiedResource: templateapi.Resource("brokertemplateinstances"),

		CreateStrategy: rest.Strategy,
		UpdateStrategy: rest.Strategy,
		DeleteStrategy: rest.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: rest.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
