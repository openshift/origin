package etcd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	imageadmission "github.com/openshift/origin/pkg/image/admission"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// REST implements a RESTStorage for image streams against etcd.
type REST struct {
	*registry.Store
	subjectAccessReviewRegistry subjectaccessreview.Registry
}

// NewREST returns a new REST.
func NewREST(optsGetter restoptions.Getter, defaultRegistry imageapi.DefaultRegistry, subjectAccessReviewRegistry subjectaccessreview.Registry, limitVerifier imageadmission.LimitVerifier) (*REST, *StatusREST, *InternalREST, error) {
	store := registry.Store{
		Copier:            kapi.Scheme,
		NewFunc:           func() runtime.Object { return &imageapi.ImageStream{} },
		NewListFunc:       func() runtime.Object { return &imageapi.ImageStreamList{} },
		PredicateFunc:     imagestream.Matcher,
		QualifiedResource: imageapi.Resource("imagestreams"),
	}

	rest := &REST{
		Store: &store,
		subjectAccessReviewRegistry: subjectAccessReviewRegistry,
	}
	// strategy must be able to load image streams across namespaces during tag verification
	strategy := imagestream.NewStrategy(defaultRegistry, subjectAccessReviewRegistry, limitVerifier, rest)

	store.CreateStrategy = strategy
	store.UpdateStrategy = strategy
	store.DeleteStrategy = strategy
	store.Decorator = strategy.Decorate

	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: imagestream.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, nil, nil, err
	}

	statusStrategy := imagestream.NewStatusStrategy(strategy)
	statusStore := store
	statusStore.Decorator = nil
	statusStore.CreateStrategy = nil
	statusStore.UpdateStrategy = statusStrategy
	statusREST := &StatusREST{store: &statusStore}

	internalStore := store
	internalStrategy := imagestream.NewInternalStrategy(strategy)
	internalStore.Decorator = nil
	internalStore.CreateStrategy = internalStrategy
	internalStore.UpdateStrategy = internalStrategy

	internalREST := &InternalREST{store: &internalStore}
	return rest, statusREST, internalREST, nil
}

// StatusREST implements the REST endpoint for changing the status of an image stream.
type StatusREST struct {
	store *registry.Store
}

// StatusREST implements Patcher
var _ = rest.Patcher(&StatusREST{})

func (r *StatusREST) New() runtime.Object {
	return &imageapi.ImageStream{}
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo)
}

// InternalREST implements the REST endpoint for changing both the spec and status of an image stream.
type InternalREST struct {
	store *registry.Store
}

func (r *InternalREST) New() runtime.Object {
	return &imageapi.ImageStream{}
}

// Create alters both the spec and status of the object.
func (r *InternalREST) Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error) {
	return r.store.Create(ctx, obj, false)
}

// Update alters both the spec and status of the object.
func (r *InternalREST) Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo)
}
