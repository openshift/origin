package image

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/dockerregistry"
	imageapi "github.com/openshift/origin/pkg/image/api"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

// NewImageStreamEvaluator computes resource usage of ImageStreams. Instantiating this is necessary for image
// resource quota admission controller to properly work on ImageStreamMapping objects. Project image size
// usage must be evaluated in quota's usage status before a CREATE operation on ImageStreamMapping can be
// allowed.
func NewImageStreamEvaluator(osClient osclient.Interface, registryClientFactory quotautil.InternalRegistryClientFactory) kquota.Evaluator {
	computeResources := []kapi.ResourceName{
		imageapi.ResourceProjectImagesSize,
		//  Used values need to be set on resource quota before admission controller can handle requests.
		//  Therefor we return following resources as well. Even though we evaluate them always to 0.
		imageapi.ResourceImageStreamSize,
		imageapi.ResourceImageSize,
	}

	matchesScopeFunc := func(kapi.ResourceQuotaScope, runtime.Object) bool { return true }
	getFuncByNamespace := func(namespace, name string) (runtime.Object, error) {
		return osClient.ImageStreams(namespace).Get(name)
	}
	listFuncByNamespace := func(namespace string, options kapi.ListOptions) (runtime.Object, error) {
		return osClient.ImageStreams(namespace).List(options)
	}

	return quotautil.NewSharedContextEvaluator(
		"ImageStream evaluator",
		kapi.Kind("ImageStream"),
		map[admission.Operation][]kapi.ResourceName{
			admission.Create: computeResources,
			admission.Update: computeResources,
		},
		matchesScopeFunc,
		getFuncByNamespace,
		listFuncByNamespace,
		imageStreamConstraintsFunc,
		makeImageStreamUsageComputerFactory(osClient, registryClientFactory))
}

// imageStreamConstraintsFunc checks that given object is an image stream
func imageStreamConstraintsFunc(required []kapi.ResourceName, object runtime.Object) error {
	if _, ok := object.(*imageapi.ImageStream); !ok {
		return fmt.Errorf("Unexpected input object %v", object)
	}
	return nil
}

// makeImageStreamUsageComputerFactory returns an object used during computation of image quota across all
// repositories in a namespace.
func makeImageStreamUsageComputerFactory(osClient osclient.Interface, rcFactory quotautil.InternalRegistryClientFactory) quotautil.UsageComputerFactory {
	return func() quotautil.UsageComputer {
		return &imageStreamUsageComputer{
			osClient:         osClient,
			rcFactory:        rcFactory,
			imageCache:       make(map[string]*imageapi.Image),
			cachedLayerSizes: make(map[string]int64),
		}
	}
}

// imageStreamUsageComputer is a context object for use in SharedContextEvaluator.
type imageStreamUsageComputer struct {
	osClient  osclient.Interface
	rcFactory quotautil.InternalRegistryClientFactory
	rClient   dockerregistry.Client
	// imageCache maps image name to an image object. It holds only images
	// stored in the registry to avoid multiple fetches of the same object.
	imageCache map[string]*imageapi.Image
	// Maps layer names to their sizes fetched from internal registry.
	cachedLayerSizes map[string]int64
}

// Usage returns a usage for an image stream. The only resource computed is
// ResourceProjectImagesSize which is the only resource scoped to a namespace.
func (c *imageStreamUsageComputer) Usage(object runtime.Object) kapi.ResourceList {
	is, ok := object.(*imageapi.ImageStream)
	if !ok {
		return kapi.ResourceList{}
	}

	res := map[kapi.ResourceName]resource.Quantity{
		imageapi.ResourceProjectImagesSize: *c.getImageStreamSize(is),
		imageapi.ResourceImageStreamSize:   *resource.NewQuantity(0, resource.BinarySI),
		imageapi.ResourceImageSize:         *resource.NewQuantity(0, resource.BinarySI),
	}

	return res
}

// getImageStreamSize computes a sum of sizes of image layers occupying given image stream. Each layer
// is added just once even if it occurs in multiple images.
func (c *imageStreamUsageComputer) getImageStreamSize(is *imageapi.ImageStream) *resource.Quantity {
	size := resource.NewQuantity(0, resource.BinarySI)

	processedImages := make(map[string]sets.Empty)
	processedLayers := make(map[string]sets.Empty)

	for _, history := range is.Status.Tags {
		for i := range history.Items {
			imgName := history.Items[i].Image
			if len(history.Items[i].DockerImageReference) == 0 || len(imgName) == 0 {
				continue
			}

			if _, exists := processedImages[imgName]; exists {
				continue
			}
			processedImages[imgName] = sets.Empty{}

			img, exists := c.imageCache[imgName]
			if !exists {
				imi, err := c.osClient.ImageStreamImages(is.Namespace).Get(is.Name, imgName)
				if err != nil {
					glog.Errorf("Failed to get image %s of image stream %s/%s: %v", imgName, is.Namespace, is.Name, err)
					continue
				}
				img = &imi.Image
				c.imageCache[imgName] = img
			}

			if value, ok := img.Annotations[imageapi.ManagedByOpenShiftAnnotation]; !ok || value != "true" {
				glog.V(5).Infof("Image %q with DockerImageReference %q belongs to an external registry - skipping", img.Name, img.DockerImageReference)
				continue
			}

			if len(img.DockerImageLayers) == 0 || img.DockerImageMetadata.Size == 0 {
				if err := c.loadImageLayerSizes(is.Namespace, is.Name, history.Items[i].DockerImageReference, img); err != nil {
					glog.Errorf("Failed to load layer sizes of image %s: %v", img.Name, err)
					continue
				}
			}

			for _, layer := range img.DockerImageLayers {
				if _, ok := processedLayers[layer.Name]; ok {
					continue
				}
				size.Add(*resource.NewQuantity(layer.Size, resource.BinarySI))
				processedLayers[layer.Name] = sets.Empty{}
			}
		}
	}

	return size
}

// loadImageLayerSizes sets metadata to the given image. It will also query sizes of image's layers and
// sets them on image object.
func (c *imageStreamUsageComputer) loadImageLayerSizes(namespace, name, dockerImageReference string, image *imageapi.Image) error {
	if err := imageapi.ImageWithMetadata(image); err != nil {
		return err
	}

	for _, layer := range image.DockerImageLayers {
		if layer.Size != 0 {
			return nil
		}
	}

	glog.V(4).Infof("all layers of %s have zero size, fetching sizes from internal Docker registry", dockerImageReference)

	conn, err := c.getRegistryConnection(dockerImageReference)
	if err != nil {
		glog.Errorf("failed to create a connection to registry from docker image reference %q: %v", dockerImageReference, err)
		return nil
	}

	var totalSize int64
	for i := range image.DockerImageLayers {
		layer := &image.DockerImageLayers[i]
		if size, exists := c.cachedLayerSizes[layer.Name]; exists {
			if size != 0 {
				layer.Size = size
			}
			totalSize += layer.Size
			continue
		}
		exists, size, err := conn.BlobExists(namespace, name, layer.Name)
		if err != nil {
			glog.V(4).Infof("failed to check existence of layer %q in %s/%s: %v", layer.Name, namespace, name, err)
			continue
		}

		c.cachedLayerSizes[layer.Name] = size

		if !exists {
			glog.V(4).Infof("layer %q does not exist in internal registry under %s/%s", namespace, name)
			continue
		}
		layer.Size = size
		totalSize += layer.Size
	}

	// TODO: update image to avoid re-fetching sizes
	image.DockerImageMetadata.Size = totalSize

	return nil
}

// getRegistryConnection returns registry connection to internal registry. Registry url
// must be stored in given dockerImageReference. The connection object is cached.
func (c *imageStreamUsageComputer) getRegistryConnection(dockerImageReference string) (dockerregistry.Connection, error) {
	parsed, err := imageapi.ParseDockerImageReference(dockerImageReference)
	if err != nil {
		return nil, err
	}
	if len(parsed.Registry) == 0 {
		return nil, fmt.Errorf("missing registry in imageReference")
	}

	if c.rClient == nil {
		c.rClient, err = c.rcFactory.GetClient()
		if err != nil {
			return nil, err
		}
	}

	return c.rClient.Connect(parsed.Registry, true)
}
