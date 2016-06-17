package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// REST implements a RESTStorage for images against etcd.
type REST struct {
	*registry.Store
}

// NewREST returns a new REST.
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	prefix := "/images"

	store := &registry.Store{
		NewFunc: func() runtime.Object { return &api.Image{} },

		// NewListFunc returns an object capable of storing results of an etcd list.
		NewListFunc: func() runtime.Object { return &api.ImageList{} },
		// Produces a path that etcd understands, to the root of the resource
		// by combining the namespace in the context with the given prefix.
		// Yet images are not namespace scoped, so we're returning just prefix here.
		KeyRootFunc: func(ctx kapi.Context) string {
			return prefix
		},
		// Produces a path that etcd understands, to the resource by combining
		// the namespace in the context with the given prefix
		// Yet images are not namespace scoped, so we're returning just prefix here.
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return registry.NoNamespaceKeyFunc(ctx, prefix, name)
		},
		// Retrieve the name field of an image
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.Image).Name, nil
		},
		// Used to match objects based on labels/fields for list and watch
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return image.MatchImage(label, field)
		},
		QualifiedResource: api.Resource("images"),

		// Used to validate image creation
		CreateStrategy: image.Strategy,

		// Used to validate image updates
		UpdateStrategy: image.Strategy,

		ReturnDeletedObject: false,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, prefix); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
