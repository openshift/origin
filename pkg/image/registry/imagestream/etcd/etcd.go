package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// REST implements a RESTStorage for image streams against etcd.
type REST struct {
	*etcdgeneric.Etcd
	subjectAccessReviewRegistry subjectaccessreview.Registry
}

// NewREST returns a new REST.
func NewREST(optsGetter restoptions.Getter, defaultRegistry imagestream.DefaultRegistry, subjectAccessReviewRegistry subjectaccessreview.Registry) (*REST, *StatusREST, *InternalREST, error) {
	prefix := "/imagestreams"

	store := etcdgeneric.Etcd{
		NewFunc: func() runtime.Object { return &api.ImageStream{} },

		// NewListFunc returns an object capable of storing results of an etcd list.
		NewListFunc: func() runtime.Object { return &api.ImageStreamList{} },
		// Produces a path that etcd understands, to the root of the resource
		// by combining the namespace in the context with the given prefix.
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdgeneric.NamespaceKeyRootFunc(ctx, prefix)
		},
		// Produces a path that etcd understands, to the resource by combining
		// the namespace in the context with the given prefix
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return etcdgeneric.NamespaceKeyFunc(ctx, prefix, name)
		},
		// Retrieve the name field of an image
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.ImageStream).Name, nil
		},
		// Used to match objects based on labels/fields for list and watch
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return imagestream.MatchImageStream(label, field)
		},
		QualifiedResource: api.Resource("imagestreams"),

		ReturnDeletedObject: false,
	}

	strategy := imagestream.NewStrategy(defaultRegistry, subjectAccessReviewRegistry)
	rest := &REST{Etcd: &store, subjectAccessReviewRegistry: subjectAccessReviewRegistry}
	strategy.ImageStreamGetter = rest

	store.CreateStrategy = strategy
	store.UpdateStrategy = strategy
	store.Decorator = strategy.Decorate

	if err := restoptions.ApplyOptions(optsGetter, &store, prefix); err != nil {
		return nil, nil, nil, err
	}

	statusStore := store
	statusStore.Decorator = nil
	statusStore.CreateStrategy = nil
	statusStore.UpdateStrategy = imagestream.NewStatusStrategy(strategy)

	internalStore := store
	internalStrategy := imagestream.NewInternalStrategy(strategy)
	internalStore.Decorator = nil
	internalStore.CreateStrategy = internalStrategy
	internalStore.UpdateStrategy = internalStrategy

	return rest, &StatusREST{store: &statusStore}, &InternalREST{store: &internalStore}, nil
}

// StatusREST implements the REST endpoint for changing the status of an image stream.
type StatusREST struct {
	store *etcdgeneric.Etcd
}

func (r *StatusREST) New() runtime.Object {
	return &api.ImageStream{}
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return r.store.Update(ctx, obj)
}

// InternalREST implements the REST endpoint for changing both the spec and status of an image stream.
type InternalREST struct {
	store *etcdgeneric.Etcd
}

func (r *InternalREST) New() runtime.Object {
	return &api.ImageStream{}
}

// Create alters both the spec and status of the object.
func (r *InternalREST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	return r.store.Create(ctx, obj)
}

// Update alters both the spec and status of the object.
func (r *InternalREST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return r.store.Update(ctx, obj)
}
