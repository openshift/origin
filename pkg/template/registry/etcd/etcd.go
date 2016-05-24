package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/template/api"
	template "github.com/openshift/origin/pkg/template/registry"
)

// REST implements a RESTStorage for templates against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against templates.
func NewREST(opts generic.RESTOptions) *REST {
	prefix := "/templates"

	newListFunc := func() runtime.Object { return &api.TemplateList{} }
	storageInterface := opts.Decorator(opts.Storage, 100, &api.TemplateList{}, prefix, template.Strategy, newListFunc)

	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.Template{} },
		NewListFunc: newListFunc,
		KeyRootFunc: func(ctx kapi.Context) string {
			return registry.NamespaceKeyRootFunc(ctx, prefix)
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return registry.NamespaceKeyFunc(ctx, prefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.Template).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return template.Matcher(label, field)
		},
		QualifiedResource: api.Resource("templates"),

		CreateStrategy: template.Strategy,
		UpdateStrategy: template.Strategy,

		ReturnDeletedObject: true,

		Storage: storageInterface,
	}

	return &REST{store}
}
