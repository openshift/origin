package etcd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	authorizationclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"

	imageadmission "github.com/openshift/origin/pkg/image/admission"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/validation/whitelist"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// REST implements a RESTStorage for image streams against etcd.
type REST struct {
	*registry.Store
}

var _ rest.StandardStorage = &REST{}
var _ rest.ShortNamesProvider = &REST{}
var _ rest.CategoriesProvider = &REST{}

// Categories implements the CategoriesProvider interface. Returns a list of categories a resource is part of.
func (r *REST) Categories() []string {
	return []string{"all"}
}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"is"}
}

// NewREST returns a new REST.
func NewREST(
	optsGetter restoptions.Getter,
	registryHostname imageapi.RegistryHostnameRetriever,
	subjectAccessReviewRegistry authorizationclient.SubjectAccessReviewInterface,
	limitVerifier imageadmission.LimitVerifier,
	registryWhitelister whitelist.RegistryWhitelister,
) (*REST, *StatusREST, *InternalREST, error) {
	store := registry.Store{
		NewFunc:                  func() runtime.Object { return &imageapi.ImageStream{} },
		NewListFunc:              func() runtime.Object { return &imageapi.ImageStreamList{} },
		DefaultQualifiedResource: imageapi.Resource("imagestreams"),
	}

	rest := &REST{
		Store: &store,
	}
	// strategy must be able to load image streams across namespaces during tag verification
	strategy := imagestream.NewStrategy(registryHostname, subjectAccessReviewRegistry, limitVerifier, registryWhitelister, rest)

	store.CreateStrategy = strategy
	store.UpdateStrategy = strategy
	store.DeleteStrategy = strategy
	store.Decorator = strategy.Decorate

	options := &generic.StoreOptions{
		RESTOptions: optsGetter,
		AttrFunc:    storage.AttrFunc(storage.DefaultNamespaceScopedAttr).WithFieldMutation(imageapi.ImageStreamSelector),
	}
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

var _ rest.Getter = &StatusREST{}
var _ rest.Updater = &StatusREST{}

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
func (r *StatusREST) Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation)
}

// InternalREST implements the REST endpoint for changing both the spec and status of an image stream.
type InternalREST struct {
	store *registry.Store
}

var _ rest.Creater = &InternalREST{}
var _ rest.Updater = &InternalREST{}

func (r *InternalREST) New() runtime.Object {
	return &imageapi.ImageStream{}
}

// Create alters both the spec and status of the object.
func (r *InternalREST) Create(ctx apirequest.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, _ bool) (runtime.Object, error) {
	return r.store.Create(ctx, obj, createValidation, false)
}

// Update alters both the spec and status of the object.
func (r *InternalREST) Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation)
}

// LegacyREST allows us to wrap and alter some behavior
type LegacyREST struct {
	*REST
}

func (r *LegacyREST) Categories() []string {
	return []string{}
}
