package image

import (
	"fmt"
	"strings"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/resource"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/runtime"

	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

const imageStreamTagEvaluatorName = "Evaluator.ImageStreamTag"

// NewImageStreamTagEvaluator computes resource usage of ImageStreamsTags. Its sole purpose is to handle
// UPDATE admission operations on imageStreamTags resource.
func NewImageStreamTagEvaluator(osClient osclient.Interface) kquota.Evaluator {
	computeResources := []kapi.ResourceName{
		imageapi.ResourceImages,
	}

	matchesScopeFunc := func(kapi.ResourceQuotaScope, runtime.Object) bool { return true }
	getFuncByNamespace := func(namespace, id string) (runtime.Object, error) {
		nameParts := strings.SplitN(id, ":", 2)
		if len(nameParts) != 2 {
			return nil, fmt.Errorf("%q is an invalid id for an imagestreamtag. Must be in form <name>:<tag>.", id)
		}

		obj, err := osClient.ImageStreamTags(namespace).Get(nameParts[0], nameParts[1])
		if err != nil {
			if !kerrors.IsNotFound(err) {
				return nil, err
			}
			obj = &imageapi.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: namespace,
					Name:      id,
				},
			}
		}
		return obj, nil
	}

	return quotautil.NewSharedContextEvaluator(
		imageStreamTagEvaluatorName,
		kapi.Kind("ImageStreamTag"),
		map[admission.Operation][]kapi.ResourceName{admission.Update: computeResources},
		computeResources,
		matchesScopeFunc,
		getFuncByNamespace,
		nil,
		imageStreamTagConstraintsFunc,
		makeImageStreamTagUsageComputerFactory(osClient))
}

// imageStreamTagConstraintsFunc checks that given object is an image stream tag
func imageStreamTagConstraintsFunc(required []kapi.ResourceName, object runtime.Object) error {
	if _, ok := object.(*imageapi.ImageStreamTag); !ok {
		return fmt.Errorf("Unexpected input object %v", object)
	}
	return nil
}

// makeImageStreamTagUsageComputerFactory returns an object used during computation of image quota across all
// repositories in a namespace.
func makeImageStreamTagUsageComputerFactory(osClient osclient.Interface) quotautil.UsageComputerFactory {
	return func() quotautil.UsageComputer {
		return &imageStreamTagUsageComputer{
			GenericImageStreamUsageComputer: *NewGenericImageStreamUsageComputer(osClient, false, true),
		}
	}
}

// imageStreamUsageComputer is a context object for use in SharedContextEvaluator.
type imageStreamTagUsageComputer struct {
	GenericImageStreamUsageComputer
}

// Usage returns a usage for an image stream tag.
func (c *imageStreamTagUsageComputer) Usage(object runtime.Object) kapi.ResourceList {
	ist, ok := object.(*imageapi.ImageStreamTag)
	if !ok {
		return kapi.ResourceList{}
	}

	res := map[kapi.ResourceName]resource.Quantity{
		imageapi.ResourceImages: *resource.NewQuantity(0, resource.BinarySI),
	}

	if ist.Tag == nil {
		glog.V(4).Infof("Nothing to tag to %s/%s", ist.Namespace, ist.Name)
		return res
	}

	nameParts := strings.Split(ist.Name, ":")
	if len(nameParts) != 2 {
		glog.Errorf("failed to parse name of image stream tag %q", ist.Name)
		return kapi.ResourceList{}
	}

	if ist.Tag.From == nil {
		glog.V(2).Infof("from unspecified in tag reference of istag %s/%s, skipping", ist.Namespace, ist.Name)
		return res
	}

	ref, err := c.getImageReferenceForObjectReference(ist.Namespace, ist.Tag.From)
	if err != nil {
		glog.Errorf("failed to get source docker image reference for istag %s/%s: %v", ist.Namespace, nameParts[0], err)
		return res
	}

	img, err := c.getImage(ref.ID)
	if err != nil {
		glog.Errorf("failed to get an image %s: %v", ref.ID, err)
		return res
	}

	_, imagesIncrement, err := c.GetProjectImagesUsageIncrement(ist.Namespace, nil, img)
	if err != nil {
		glog.Errorf("Failed to get namespace size increment of %q with an image %q: %v", ist.Namespace, img.Name, err)
		return res
	}

	res[imageapi.ResourceImages] = *imagesIncrement

	return res
}
