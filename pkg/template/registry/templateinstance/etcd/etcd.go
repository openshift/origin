package etcd

import (
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/template/api"
	"github.com/openshift/origin/pkg/template/registry/templateinstance"
	"github.com/openshift/origin/pkg/util/restoptions"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
)

// REST implements a RESTStorage for templateinstances against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against templateinstances.
func NewREST(optsGetter restoptions.Getter, oc *client.Client) (*REST, error) {
	strategy := templateinstance.NewStrategy(oc)

	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.TemplateInstance{} },
		NewListFunc:       func() runtime.Object { return &api.TemplateInstanceList{} },
		PredicateFunc:     templateinstance.Matcher,
		QualifiedResource: api.Resource("templateinstances"),

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
