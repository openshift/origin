package etcd

import (
	"fmt"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	imagev1 "github.com/openshift/api/image/v1"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// ImageLayerIndex is a cache of image digests to the layers they contain.
// Because a very large number of images can exist on a cluster, we only
// hold in memory a small subset of the full image object.
type ImageLayerIndex interface {
	HasSynced() bool
	GetByKey(key string) (item interface{}, exists bool, err error)
	Run(stopCh <-chan struct{})
}

type ImageListWatch interface {
	List(metav1.ListOptions) (*imagev1.ImageList, error)
	Watch(metav1.ListOptions) (watch.Interface, error)
}

type imageLayerIndex struct {
	informer cache.SharedIndexInformer
}

func NewEmptyLayerIndex() ImageLayerIndex {
	return imageLayerIndex{}
}

func (i imageLayerIndex) HasSynced() bool {
	if i.informer == nil {
		return true
	}
	return i.informer.HasSynced()
}
func (i imageLayerIndex) GetByKey(key string) (item interface{}, exists bool, err error) {
	if i.informer == nil {
		return nil, false, nil
	}
	return i.informer.GetStore().GetByKey(key)
}
func (i imageLayerIndex) Run(stopCh <-chan struct{}) {
	if i.informer == nil {
		return
	}
	i.informer.Run(stopCh)
}

// NewImageLayerIndex creates a new index over a store that must return
// images.
func NewImageLayerIndex(lw ImageListWatch) ImageLayerIndex {
	informer := cache.NewSharedIndexInformer(&cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			list, err := lw.List(metav1.ListOptions{
				ResourceVersion: options.ResourceVersion,
				Limit:           options.Limit,
				Continue:        options.Continue,
			})
			if err != nil {
				return nil, err
			}
			// reduce the full image list to a smaller subset.
			out := &metainternalversion.List{
				ListMeta: metav1.ListMeta{
					Continue:        list.Continue,
					ResourceVersion: list.ResourceVersion,
				},
				Items: make([]runtime.Object, len(list.Items)),
			}
			for i, image := range list.Items {
				out.Items[i] = imageLayersForImage(&image)
			}
			return out, nil
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			w, err := lw.Watch(metav1.ListOptions{
				ResourceVersion: options.ResourceVersion,
			})
			if err != nil {
				return nil, err
			}
			return watch.Filter(w, func(in watch.Event) (out watch.Event, keep bool) {
				if in.Object == nil {
					return in, true
				}
				// reduce each object to the minimal subset we need for the cache
				image, ok := in.Object.(*imagev1.Image)
				if !ok {
					return in, true
				}
				in.Object = imageLayersForImage(image)
				return in, true
			}), nil
		},
	}, &ImageLayers{}, 0, cache.Indexers{
		// layers allows fast access to the images with a given layer
		"layers": func(obj interface{}) ([]string, error) {
			entry, ok := obj.(*ImageLayers)
			if !ok {
				return nil, fmt.Errorf("unexpected cache object %T", obj)
			}
			keys := make([]string, 0, len(entry.Layers))
			for _, layer := range entry.Layers {
				keys = append(keys, layer.Name)
			}
			return keys, nil
		},
	})
	return imageLayerIndex{informer: informer}
}

// configFromImage attempts to find a config blob description from
// an image. Images older than schema2 in Docker do not have a config blob - the manifest
// has that data embedded.
func configFromImage(image *imagev1.Image) *imagev1.ImageLayer {
	if image.DockerImageManifestMediaType != "application/vnd.docker.distribution.manifest.v2+json" {
		return nil
	}
	meta := &imageapi.DockerImage{}
	if _, _, err := legacyscheme.Codecs.UniversalDecoder().Decode(image.DockerImageMetadata.Raw, nil, meta); err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to decode image for layer cache: %v", err))
		return nil
	}
	return &imagev1.ImageLayer{
		Name:      meta.ID,
		MediaType: "application/vnd.docker.container.image.v1+json",
	}
}

// ImageLayers is the minimal set of data we need to retain to provide the cache.
// Unlike a more general informer cache, we do not retain the full object because of
// the potential size of the objects being stored. Even a small cluster may have 20k
// or more images in active use.
type ImageLayers struct {
	Name            string
	ResourceVersion string
	MediaType       string
	Config          *imagev1.ImageLayer
	Layers          []imagev1.ImageLayer
}

func imageLayersForImage(image *imagev1.Image) *ImageLayers {
	mediaType := image.DockerImageManifestMediaType
	if len(mediaType) == 0 {
		mediaType = "application/vnd.docker.distribution.manifest.v2+json"
	}
	return &ImageLayers{
		Name:            image.Name,
		ResourceVersion: image.ResourceVersion,
		MediaType:       mediaType,
		Config:          configFromImage(image),
		Layers:          image.DockerImageLayers,
	}
}

var (
	_ runtime.Object = &ImageLayers{}
	_ metav1.Object  = &ImageLayers{}
)

func (l *ImageLayers) GetObjectKind() schema.ObjectKind { return &metav1.TypeMeta{} }
func (l *ImageLayers) DeepCopyObject() runtime.Object {
	var layers []imagev1.ImageLayer
	if l.Layers != nil {
		layers = make([]imagev1.ImageLayer, len(l.Layers))
		copy(layers, l.Layers)
	}
	var config *imagev1.ImageLayer
	if l.Config != nil {
		copied := *l.Config
		config = &copied
	}
	return &ImageLayers{
		Name:            l.Name,
		ResourceVersion: l.ResourceVersion,
		MediaType:       l.MediaType,
		Config:          config,
		Layers:          layers,
	}
}

// client-go/cache.SharedIndexInformer hardcodes the key function to assume ObjectMeta.
// Here we implement the relevant accessors to allow a minimal index to be created.
// SharedIndexInformer will be refactored to require a more minimal subset of actions
// in the near future.

func (l *ImageLayers) GetName() string                   { return l.Name }
func (l *ImageLayers) GetNamespace() string              { return "" }
func (l *ImageLayers) GetResourceVersion() string        { return l.ResourceVersion }
func (l *ImageLayers) SetResourceVersion(version string) { l.ResourceVersion = version }

// These methods are unused stubs to satisfy meta.Object.

func (l *ImageLayers) SetNamespace(namespace string)                     {}
func (l *ImageLayers) SetName(name string)                               {}
func (l *ImageLayers) GetGenerateName() string                           { return "" }
func (l *ImageLayers) SetGenerateName(name string)                       {}
func (l *ImageLayers) GetUID() types.UID                                 { return "" }
func (l *ImageLayers) SetUID(uid types.UID)                              {}
func (l *ImageLayers) GetGeneration() int64                              { return 0 }
func (l *ImageLayers) SetGeneration(generation int64)                    {}
func (l *ImageLayers) GetSelfLink() string                               { return "" }
func (l *ImageLayers) SetSelfLink(selfLink string)                       {}
func (l *ImageLayers) GetCreationTimestamp() metav1.Time                 { return metav1.Time{} }
func (l *ImageLayers) SetCreationTimestamp(timestamp metav1.Time)        {}
func (l *ImageLayers) GetDeletionTimestamp() *metav1.Time                { return nil }
func (l *ImageLayers) SetDeletionTimestamp(timestamp *metav1.Time)       {}
func (l *ImageLayers) GetDeletionGracePeriodSeconds() *int64             { return nil }
func (l *ImageLayers) SetDeletionGracePeriodSeconds(*int64)              {}
func (l *ImageLayers) GetLabels() map[string]string                      { return nil }
func (l *ImageLayers) SetLabels(labels map[string]string)                {}
func (l *ImageLayers) GetAnnotations() map[string]string                 { return nil }
func (l *ImageLayers) SetAnnotations(annotations map[string]string)      {}
func (l *ImageLayers) GetInitializers() *metav1.Initializers             { return nil }
func (l *ImageLayers) SetInitializers(initializers *metav1.Initializers) {}
func (l *ImageLayers) GetFinalizers() []string                           { return nil }
func (l *ImageLayers) SetFinalizers(finalizers []string)                 {}
func (l *ImageLayers) GetOwnerReferences() []metav1.OwnerReference       { return nil }
func (l *ImageLayers) SetOwnerReferences([]metav1.OwnerReference)        {}
func (l *ImageLayers) GetClusterName() string                            { return "" }
func (l *ImageLayers) SetClusterName(clusterName string)                 {}
