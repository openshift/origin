package image

import (
	"fmt"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/runtime"

	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

// NewImageStreamEvaluator computes resource usage of ImageStreams. Instantiating this is necessary for image
// resource quota admission controller to properly work on ImageStreamMapping objects. Project image size
// usage must be evaluated in quota's usage status before a CREATE operation on ImageStreamMapping can be
// allowed.
func NewImageStreamEvaluator(osClient osclient.Interface) kquota.Evaluator {
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
		makeImageStreamUsageComputerFactory(osClient))
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
func makeImageStreamUsageComputerFactory(osClient osclient.Interface) quotautil.UsageComputerFactory {
	return func() quotautil.UsageComputer {
		return &imageStreamUsageComputer{
			osClient:   osClient,
			imageCache: make(map[string]*imageapi.Image),
		}
	}
}

// imageStreamUsageComputer is a context object for use in SharedContextEvaluator.
type imageStreamUsageComputer struct {
	osClient osclient.Interface
	// imageCache maps image name to a an image object. It holds only images
	// stored in the registry to avoid multiple fetches of the same object.
	imageCache map[string]*imageapi.Image
}

// Usage returns a usage for an image stream. The only resource computed is
// ResourceProjectImagesSize which is the only resource scoped to a namespace.
func (c *imageStreamUsageComputer) Usage(object runtime.Object) kapi.ResourceList {
	is, ok := object.(*imageapi.ImageStream)
	if !ok {
		return kapi.ResourceList{}
	}

	res := map[kapi.ResourceName]resource.Quantity{
		imageapi.ResourceProjectImagesSize: *GetImageStreamSize(c.osClient, is, c.imageCache),
		imageapi.ResourceImageStreamSize:   *resource.NewQuantity(0, resource.BinarySI),
		imageapi.ResourceImageSize:         *resource.NewQuantity(0, resource.BinarySI),
	}

	return res
}
