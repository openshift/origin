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
	imageapi "github.com/openshift/origin/pkg/image/api"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

// NewImageStreamMappingEvaluator computes resource usage for ImageStreamMapping objects. This particular kind
// is a virtual resource. It depends on ImageStream usage evaluator to compute project image size accross all
// image streams in the namespace.
func NewImageStreamMappingEvaluator(osClient osclient.Interface, rcFactory quotautil.InternalRegistryClientFactory) kquota.Evaluator {
	computeResources := []kapi.ResourceName{
		imageapi.ResourceProjectImagesSize,
		imageapi.ResourceImageStreamSize,
		imageapi.ResourceImageSize,
	}

	matchesScopeFunc := func(kapi.ResourceQuotaScope, runtime.Object) bool { return true }
	listFuncByNamespace := func(namespace string, options kapi.ListOptions) (runtime.Object, error) {
		// we don't want to be called on image stream changes; ImageStreamEvaluator will do the job
		return &kapi.List{Items: []runtime.Object{}}, nil
	}

	return quotautil.NewSharedContextEvaluator(
		"ImageStreamMapping evaluator",
		kapi.Kind("ImageStreamMapping"),
		map[admission.Operation][]kapi.ResourceName{admission.Create: computeResources},
		matchesScopeFunc,
		nil,
		listFuncByNamespace,
		imageStreamMappingConstraintsFunc,
		makeImageStreamMappingUsageComputerFactory(osClient, rcFactory),
	)
}

// imageStreamMappingConstraintsFunc checks that given object is an image stream
func imageStreamMappingConstraintsFunc(required []kapi.ResourceName, object runtime.Object) error {
	if _, ok := object.(*imageapi.ImageStreamMapping); !ok {
		return fmt.Errorf("Unexpected input object %v", object)
	}
	return nil
}

func makeImageStreamMappingUsageComputerFactory(osClient osclient.Interface, rcFactory quotautil.InternalRegistryClientFactory) quotautil.UsageComputerFactory {
	return func() quotautil.UsageComputer {
		isuc := &imageStreamUsageComputer{
			osClient:         osClient,
			rcFactory:        rcFactory,
			imageCache:       make(map[string]*imageapi.Image),
			cachedLayerSizes: make(map[string]int64),
		}

		return &imageStreamMappingUsageComputer{
			imageStreamUsageComputer: isuc,
			imageStreamCache:         make(map[string]*imageapi.ImageStream),
		}
	}
}

// imageStreamMappingUsageComputer is used to compute usage for ImageStreamMapping.
type imageStreamMappingUsageComputer struct {
	*imageStreamUsageComputer

	// imageStreamCache maps image stream names to image stream objects. Used in case of evaluation of all
	// ImageStreamMapping object in a namespace. Which will hardly ever happen unless list function returns
	// non-empty lists.
	imageStreamCache map[string]*imageapi.ImageStream
}

// Usage computes usage of image stream mapping objects. There are two scenaries when this can be called:
//
//	1. admission check for CREATE on ImageStreamMapping object
//  2. evaluation of image stream mapping objects for a namespace triggered by resource quota controller
//
// In the former case, we are expected to return size increments for all the ressources. For image size and
// image stream size it means to return their whole size because their quota usage is always 0.
//
// In the latter case, we return only project images size which is scoped to namespace and thus its usage
// should be accumulated. This scenario actually shouldn't happen because it is covered by image stream
// evaluator.
func (c *imageStreamMappingUsageComputer) Usage(object runtime.Object) kapi.ResourceList {
	ism, ok := object.(*imageapi.ImageStreamMapping)
	if !ok {
		return kapi.ResourceList{}
	}

	var isSize *resource.Quantity
	var err error
	is, isLoaded := c.imageStreamCache[ism.Name]

	res := map[kapi.ResourceName]resource.Quantity{
		imageapi.ResourceProjectImagesSize: *resource.NewQuantity(0, resource.BinarySI),
		imageapi.ResourceImageStreamSize:   *resource.NewQuantity(0, resource.BinarySI),
		imageapi.ResourceImageSize:         *resource.NewQuantity(0, resource.BinarySI),
	}

	if !isLoaded {
		is, err = c.osClient.ImageStreams(ism.Namespace).Get(ism.Name)
		if err != nil {
			glog.Errorf("Failed to get image stream %s/%s: %v", ism.Namespace, ism.Name, err)
			return map[kapi.ResourceName]resource.Quantity{}
		}
		isSize = c.getImageStreamSize(is)
	}

	img, imgLoaded := c.imageCache[ism.Image.Name]

	if isLoaded && imgLoaded {
		// quota evaluation; image stream has been already processed
		return res
	}

	if !imgLoaded {
		// handling CREATE admission check - image is about to be created
		img = &ism.Image

		if value, exists := img.Annotations[imageapi.ManagedByOpenShiftAnnotation]; !exists || value != "true" {
			return res
		}

		_, sizeIncrement, err2 := c.getImageStreamSizeIncrement(is, img)
		if err2 != nil {
			glog.Errorf("Failed to get repository size increment of %s/%s with an image %q: %v", ism.Namespace, ism.Name, img.Name, err2)
			return map[kapi.ResourceName]resource.Quantity{}
		}

		// Values for repository size and image size resources don't accumulate, that's why we return whole usage.
		isSize.Add(*sizeIncrement)
		res[imageapi.ResourceImageStreamSize] = *isSize
		res[imageapi.ResourceImageSize] = *resource.NewQuantity(img.DockerImageMetadata.Size, resource.BinarySI)
		// Caller expects us to return usage increments for the CREATE operation.
		res[imageapi.ResourceProjectImagesSize] = *sizeIncrement

	} else {
		// handling quota evaluation
		res[imageapi.ResourceProjectImagesSize] = *isSize
	}

	return res
}

// getImageStreamSizeIncrement computes a sum of sizes of image layers occupying given image stream.
// Additionally it returns an increment which will be added to the size when the given image will be tagged
// into the image stream. In case the is is nil, returned size will be 0 and returned increment will equal to
// the image size.
//
// Size increment will always be less or equal to the image size. Less means, that some of image's layers are
// already stored in a registry.
func (c *imageStreamMappingUsageComputer) getImageStreamSizeIncrement(is *imageapi.ImageStream, image *imageapi.Image) (isSize, sizeIncrement *resource.Quantity, err error) {

	if len(image.DockerImageLayers) == 0 || image.DockerImageMetadata.Size == 0 {
		if err := c.loadImageLayerSizes(is.Namespace, is.Name, image.DockerImageReference, image); err != nil {
			glog.Errorf("Failed to load layer sizes of image %s: %v", image.Name, err)
			return nil, nil, err
		}
	}
	isSize = resource.NewQuantity(0, resource.BinarySI)
	if value, ok := image.Annotations[imageapi.ManagedByOpenShiftAnnotation]; ok && value == "true" {
		sizeIncrement = resource.NewQuantity(image.DockerImageMetadata.Size, resource.BinarySI)
	} else {
		sizeIncrement = resource.NewQuantity(0, resource.BinarySI)
	}
	if is == nil {
		return
	}

	processedImages := make(map[string]sets.Empty)
	processedLayers := make(map[string]sets.Empty)

	for _, history := range is.Status.Tags {
		for i := range history.Items {
			imageName := history.Items[i].Image
			if len(history.Items[i].DockerImageReference) == 0 || len(imageName) == 0 {
				continue
			}

			if _, exists := processedImages[imageName]; exists {
				continue
			}
			processedImages[imageName] = sets.Empty{}

			imi, err2 := c.osClient.ImageStreamImages(is.Namespace).Get(is.Name, imageName)
			if err2 != nil {
				glog.Errorf("Failed to get image %s of image stream %s/%s: %v", image.Name, is.Namespace, is.Name, err2)
				continue
			}
			img := &imi.Image

			if value, ok := img.Annotations[imageapi.ManagedByOpenShiftAnnotation]; !ok || value != "true" {
				glog.V(5).Infof("Image %q with DockerImageReference %q belongs to an external registry - skipping", img.Name, img.DockerImageReference)
				continue
			}

			if len(img.DockerImageLayers) == 0 || img.DockerImageMetadata.Size == 0 {
				if err2 := c.loadImageLayerSizes(is.Namespace, is.Name, history.Items[i].DockerImageReference, img); err2 != nil {
					glog.Errorf("Failed to load layer sizes of image %s: %v", img.Name, err2)
					continue
				}
			}

			for _, layer := range img.DockerImageLayers {
				if _, exists := processedLayers[layer.Name]; exists {
					continue
				}
				isSize.Add(*resource.NewQuantity(layer.Size, resource.BinarySI))
				processedLayers[layer.Name] = sets.Empty{}
			}
		}
	}

	for _, layer := range image.DockerImageLayers {
		if _, exists := processedLayers[layer.Name]; exists {
			sizeIncrement.Sub(*resource.NewQuantity(layer.Size, resource.BinarySI))
			if sizeIncrement.Value() <= 0 {
				sizeIncrement.Set(0)
				break
			}
		}
	}

	return
}
