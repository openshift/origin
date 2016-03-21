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

const (
	imageStreamEvaluatorName          = "Evaluator.ImageStream.Controller"
	imageStreamAdmissionEvaluatorName = "Evaluator.ImageStream.Admission"
)

// NewImageStreamEvaluator computes resource usage of ImageStreams. Instantiating this is necessary for
// resource quota admission controller to properly work on image stream related objects.
func NewImageStreamEvaluator(osClient osclient.Interface) kquota.Evaluator {
	computeResources := []kapi.ResourceName{
		imageapi.ResourceImages,
	}

	matchesScopeFunc := func(kapi.ResourceQuotaScope, runtime.Object) bool { return true }
	getFuncByNamespace := func(namespace, name string) (runtime.Object, error) {
		return osClient.ImageStreams(namespace).Get(name)
	}
	listFuncByNamespace := func(namespace string, options kapi.ListOptions) (runtime.Object, error) {
		return osClient.ImageStreams(namespace).List(options)
	}

	return quotautil.NewSharedContextEvaluator(
		imageStreamEvaluatorName,
		kapi.Kind("ImageStream"),
		nil,
		computeResources,
		matchesScopeFunc,
		getFuncByNamespace,
		listFuncByNamespace,
		imageStreamConstraintsFunc,
		makeImageStreamUsageComputerFactory(osClient))
}

// NewImageStreamAdmissionEvaluator computes resource usage of ImageStreams in the context of admission
// plugin.
func NewImageStreamAdmissionEvaluator(osClient osclient.Interface) kquota.Evaluator {
	evaluator := NewImageStreamEvaluator(osClient)
	isEval := evaluator.(*quotautil.SharedContextEvaluator)
	isEval.Name = imageStreamAdmissionEvaluatorName
	isEval.InternalOperationResources = map[admission.Operation][]kapi.ResourceName{
		admission.Create: isEval.MatchedResourceNames,
		admission.Update: isEval.MatchedResourceNames,
	}
	isEval.UsageComputerFactory = makeImageStreamUsageComputerForAdmissionFactory(osClient)
	// admission plugin should not attempt to list us
	isEval.ListFuncByNamespace = nil
	return isEval
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
			GenericImageStreamUsageComputer: *NewGenericImageStreamUsageComputer(osClient, false, true),
			processedImages:                 sets.NewString(),
		}
	}
}

// imageStreamUsageComputer is a context object for use in SharedContextEvaluator.
type imageStreamUsageComputer struct {
	GenericImageStreamUsageComputer
	processedImages sets.String
}

// Usage returns a usage for an image stream.
func (c *imageStreamUsageComputer) Usage(object runtime.Object) kapi.ResourceList {
	is, ok := object.(*imageapi.ImageStream)
	if !ok {
		return kapi.ResourceList{}
	}

	images := c.GetImageStreamUsage(is, c.processedImages)
	return kapi.ResourceList{
		imageapi.ResourceImages: *images,
	}
}

// makeImageStreamUsageComputerForAdmissionFactory returns an object used during computation of image quota
// across all repositories in a namespace in a context of admission plugin.
func makeImageStreamUsageComputerForAdmissionFactory(osClient osclient.Interface) quotautil.UsageComputerFactory {
	return func() quotautil.UsageComputer {
		return &imageStreamUsageComputerForAdmission{
			GenericImageStreamUsageComputer: *NewGenericImageStreamUsageComputer(osClient, true, false),
		}
	}
}

// imageStreamUsageComputerForAdmission is a context object for use in SharedContextEvaluator
type imageStreamUsageComputerForAdmission struct {
	GenericImageStreamUsageComputer
}

// Usage returns a usage for an image stream.
func (c *imageStreamUsageComputerForAdmission) Usage(object runtime.Object) kapi.ResourceList {
	is, ok := object.(*imageapi.ImageStream)
	if !ok {
		return kapi.ResourceList{}
	}

	_, imagesIncrement, err := c.GetProjectImagesUsageIncrement(is.Namespace, is, nil)
	if err != nil {
		glog.Errorf("Failed to compute project images size increment in namespace %q: %v", is.Namespace, err)
		return kapi.ResourceList{}
	}

	return map[kapi.ResourceName]resource.Quantity{
		imageapi.ResourceImages: *imagesIncrement,
	}
}
