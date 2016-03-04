package image

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/runtime"

	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

// NewImageStreamMappingEvaluator computes resource usage for ImageStreamMapping objects. This particular kind
// is a virtual resource. It depends on ImageStream usage evaluator to compute project image size accross all
// image streams in the namespace.
func NewImageStreamMappingEvaluator(osClient osclient.Interface) kquota.Evaluator {
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
		makeImageStreamMappingUsageComputerFactory(osClient),
	)
}

// imageStreamMappingConstraintsFunc checks that given object is an image stream
func imageStreamMappingConstraintsFunc(required []kapi.ResourceName, object runtime.Object) error {
	if _, ok := object.(*imageapi.ImageStreamMapping); !ok {
		return fmt.Errorf("Unexpected input object %v", object)
	}
	return nil
}

func makeImageStreamMappingUsageComputerFactory(osClient osclient.Interface) quotautil.UsageComputerFactory {
	return func() quotautil.UsageComputer {
		return &imageStreamMappingUsageComputer{
			osClient:         osClient,
			imageStreamCache: make(map[string]*imageapi.ImageStream),
			imageCache:       make(map[string]*imageapi.Image),
		}
	}
}

// imageStreamMappingUsageComputer is used to compute usage for ImageStreamMapping.
type imageStreamMappingUsageComputer struct {
	osClient osclient.Interface
	// imageStreamCache maps image stream names to image stream objects. Used in case of evaluation of all
	// ImageStreamMapping object in a namespace. Which will hardly ever happen unless list function returns
	// non-empty lists.
	imageStreamCache map[string]*imageapi.ImageStream
	// imageCache maps image names to image objects. Used to reduce repeated resource fetches.
	imageCache map[string]*imageapi.Image
}

// Usage computes usage of image stream mapping objects. There are two scenarios when this can be called:
//
//  1. admission check for CREATE on ImageStreamMapping object
//  2. evaluation of image stream mapping objects for a namespace triggered by resource quota controller
//
// In the former case, we are expected to return size increments for all the resources. For image size and
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
		isSize = GetImageStreamSize(c.osClient, is, c.imageCache)
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

		_, sizeIncrement, err2 := GetImageStreamSizeIncrement(c.osClient, is, img, c.imageCache)
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
