package etcd

import (
	"context"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	authorizationclient "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/kubernetes/pkg/printers"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"

	"github.com/openshift/api/image"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/validation/whitelist"
	imageadmission "github.com/openshift/origin/pkg/image/apiserver/admission/limitrange"
	"github.com/openshift/origin/pkg/image/apiserver/registry/imagestream"
	"github.com/openshift/origin/pkg/image/apiserver/registryhostname"
	printersinternal "github.com/openshift/origin/pkg/printers/internalversion"
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
	optsGetter generic.RESTOptionsGetter,
	registryHostname registryhostname.RegistryHostnameRetriever,
	subjectAccessReviewRegistry authorizationclient.SubjectAccessReviewInterface,
	limitVerifier imageadmission.LimitVerifier,
	registryWhitelister whitelist.RegistryWhitelister,
	imageLayerIndex ImageLayerIndex,
) (*REST, *LayersREST, *StatusREST, *InternalREST, error) {
	store := registry.Store{
		NewFunc:                  func() runtime.Object { return &imageapi.ImageStream{} },
		NewListFunc:              func() runtime.Object { return &imageapi.ImageStreamList{} },
		DefaultQualifiedResource: image.Resource("imagestreams"),

		TableConvertor: printerstorage.TableConvertor{TablePrinter: printers.NewTablePrinter().With(printersinternal.AddHandlers)},
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
		return nil, nil, nil, nil, err
	}

	layersREST := &LayersREST{index: imageLayerIndex, store: &store}

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
	return rest, layersREST, statusREST, internalREST, nil
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
func (r *StatusREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (runtime.Object, bool, error) {
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
func (r *InternalREST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, _ bool) (runtime.Object, error) {
	return r.store.Create(ctx, obj, createValidation, false)
}

// Update alters both the spec and status of the object.
func (r *InternalREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation)
}

// LayersREST implements the REST endpoint for changing both the spec and status of an image stream.
type LayersREST struct {
	store *registry.Store
	index ImageLayerIndex
}

var _ rest.Getter = &LayersREST{}

func (r *LayersREST) New() runtime.Object {
	return &imageapi.ImageStreamLayers{}
}

// Get returns the layers for an image stream.
func (r *LayersREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	if !r.index.HasSynced() {
		return nil, errors.NewServerTimeout(r.store.DefaultQualifiedResource, "get", 2)
	}
	obj, err := r.store.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	is := obj.(*imageapi.ImageStream)
	isl := &imageapi.ImageStreamLayers{
		ObjectMeta: is.ObjectMeta,
		Blobs:      make(map[string]imageapi.ImageLayerData),
		Images:     make(map[string]imageapi.ImageBlobReferences),
	}

	missing := addImageStreamLayersFromCache(isl, is, r.index)
	// if we are missing images, they may not have propogated to the cache. Wait a non-zero amount of time
	// and try again
	if len(missing) > 0 {
		// 250ms is a reasonable propagation delay for a medium to large cluster
		time.Sleep(250 * time.Millisecond)
		missing = addImageStreamLayersFromCache(isl, is, r.index)
		if len(missing) > 0 {
			// TODO: return this in the API object as well
			glog.V(2).Infof("Image stream %s/%s references %d images that could not be found: %v", is.Namespace, is.Name, len(missing), missing)
		}
	}

	return isl, nil
}

// addImageStreamLayersFromCache looks up tagged images from the provided image stream in the cache and then adds
// metadata about those images and their referenced blobs to isl. It returns the names of missing images from the
// cache.
func addImageStreamLayersFromCache(isl *imageapi.ImageStreamLayers, is *imageapi.ImageStream, index ImageLayerIndex) []string {
	var missing []string
	for _, status := range is.Status.Tags {
		for _, item := range status.Items {
			if len(item.Image) == 0 {
				continue
			}

			obj, _, _ := index.GetByKey(item.Image)
			entry, ok := obj.(*ImageLayers)
			if !ok {
				if _, ok := isl.Images[item.Image]; !ok {
					isl.Images[item.Image] = imageapi.ImageBlobReferences{ImageMissing: true}
				}
				missing = append(missing, item.Image)
				continue
			}

			// we have already added this image once
			if _, ok := isl.Images[item.Image]; ok {
				continue
			}

			var reference imageapi.ImageBlobReferences
			for _, layer := range entry.Layers {
				reference.Layers = append(reference.Layers, layer.Name)
				if _, ok := isl.Blobs[layer.Name]; !ok {
					isl.Blobs[layer.Name] = imageapi.ImageLayerData{LayerSize: &layer.LayerSize, MediaType: layer.MediaType}
				}
			}

			if blob := entry.Config; blob != nil {
				reference.Config = &blob.Name
				if _, ok := isl.Blobs[blob.Name]; !ok {
					if blob.LayerSize == 0 {
						// only send media type since we don't the size of the manifest
						isl.Blobs[blob.Name] = imageapi.ImageLayerData{MediaType: blob.MediaType}
					} else {
						isl.Blobs[blob.Name] = imageapi.ImageLayerData{LayerSize: &blob.LayerSize, MediaType: blob.MediaType}
					}
				}
			}

			// the image manifest is always a blob - schema2 images also have a config blob referenced from the manifest
			if _, ok := isl.Blobs[item.Image]; !ok {
				isl.Blobs[item.Image] = imageapi.ImageLayerData{MediaType: entry.MediaType}
			}
			isl.Images[item.Image] = reference
		}
	}
	return missing
}

// LegacyREST allows us to wrap and alter some behavior
type LegacyREST struct {
	*REST
}

func (r *LegacyREST) Categories() []string {
	return []string{}
}
