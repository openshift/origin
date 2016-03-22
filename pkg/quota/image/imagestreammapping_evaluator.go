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

const imageStreamMappingName = "Evaluator.ImageStreamMapping"

// NewImageStreamMappingEvaluator computes resource usage for ImageStreamMapping objects. This particular kind
// is a virtual resource. It depends on ImageStream usage evaluator to compute image numbers before the
// the admission can work.
func NewImageStreamMappingEvaluator(osClient osclient.Interface) kquota.Evaluator {
	computeResources := []kapi.ResourceName{
		imageapi.ResourceImages,
	}

	matchesScopeFunc := func(kapi.ResourceQuotaScope, runtime.Object) bool { return true }

	return quotautil.NewSharedContextEvaluator(
		imageStreamMappingName,
		kapi.Kind("ImageStreamMapping"),
		map[admission.Operation][]kapi.ResourceName{admission.Create: computeResources},
		computeResources,
		matchesScopeFunc,
		nil,
		nil,
		imageStreamMappingConstraintsFunc,
		makeImageStreamMappingUsageComputerFactory(osClient))
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
			GenericImageStreamUsageComputer: *NewGenericImageStreamUsageComputer(osClient, false, true),
		}
	}
}

// imageStreamMappingUsageComputer is used to compute usage for ImageStreamMapping.
type imageStreamMappingUsageComputer struct {
	GenericImageStreamUsageComputer
}

// Usage computes usage of image stream mapping objects. It is being used solely in the context of admission
// check for CREATE operation on ImageStreamMapping object.
func (c *imageStreamMappingUsageComputer) Usage(object runtime.Object) kapi.ResourceList {
	ism, ok := object.(*imageapi.ImageStreamMapping)
	if !ok {
		return kapi.ResourceList{}
	}

	_, imagesIncrement, err := c.GetProjectImagesUsageIncrement(ism.Namespace, nil, &ism.Image)
	if err != nil {
		glog.Errorf("Failed to get project images size increment of %q caused by an image %q: %v", ism.Namespace, ism.Image.Name, err)
		return map[kapi.ResourceName]resource.Quantity{}
	}

	return map[kapi.ResourceName]resource.Quantity{
		imageapi.ResourceImages: *imagesIncrement,
	}
}
