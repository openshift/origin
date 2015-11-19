package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/template/api"
	"github.com/openshift/origin/pkg/template/registry"
)

const prefix = "/templates"

// REST implements a RESTStorage for templates against etcd
type REST struct {
	*etcdgeneric.Etcd
}

// NewREST returns a RESTStorage object that will work against templates.
func NewREST(s storage.Interface) *REST {
	store := &etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.Template{} },
		NewListFunc: func() runtime.Object { return &api.TemplateList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdgeneric.NamespaceKeyRootFunc(ctx, prefix)
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return etcdgeneric.NamespaceKeyFunc(ctx, prefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.Template).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return registry.Matcher(label, field)
		},
		EndpointName: "templates",

		CreateStrategy: registry.Strategy,
		UpdateStrategy: registry.Strategy,

		ReturnDeletedObject: true,

		Storage: s,
	}
	return &REST{store}
}
