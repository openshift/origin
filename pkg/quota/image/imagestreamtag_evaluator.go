package image

import (
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kadmission "k8s.io/apiserver/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/generic"

	"github.com/openshift/api/image"
	imagev1 "github.com/openshift/api/image/v1"
	imagev1typedclient "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	imagev1lister "github.com/openshift/client-go/image/listers/image/v1"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imagev1conversions "github.com/openshift/origin/pkg/image/apis/image/v1"
)

var imageStreamTagResources = []kapi.ResourceName{
	imageapi.ResourceImageStreams,
}

type imageStreamTagEvaluator struct {
	store     imagev1lister.ImageStreamLister
	istGetter imagev1typedclient.ImageStreamTagsGetter
}

// NewImageStreamTagEvaluator computes resource usage of ImageStreamsTags. Its sole purpose is to handle
// UPDATE admission operations on imageStreamTags resource.
func NewImageStreamTagEvaluator(store imagev1lister.ImageStreamLister, istGetter imagev1typedclient.ImageStreamTagsGetter) kquota.Evaluator {
	return &imageStreamTagEvaluator{
		store:     store,
		istGetter: istGetter,
	}
}

// Constraints checks that given object is an image stream tag
func (i *imageStreamTagEvaluator) Constraints(required []kapi.ResourceName, object runtime.Object) error {
	_, okInt := object.(*imageapi.ImageStreamTag)
	_, okExt := object.(*imagev1.ImageStreamTag)
	if !okInt && !okExt {
		return fmt.Errorf("unexpected input object %v", object)
	}
	return nil
}

func (i *imageStreamTagEvaluator) GroupResource() schema.GroupResource {
	return image.Resource("imagestreamtags")
}

func (i *imageStreamTagEvaluator) Handles(a kadmission.Attributes) bool {
	operation := a.GetOperation()
	return operation == kadmission.Create || operation == kadmission.Update
}

func (i *imageStreamTagEvaluator) Matches(resourceQuota *kapi.ResourceQuota, item runtime.Object) (bool, error) {
	matchesScopeFunc := func(kapi.ScopedResourceSelectorRequirement, runtime.Object) (bool, error) { return true, nil }
	return generic.Matches(resourceQuota, item, i.MatchingResources, matchesScopeFunc)
}

func (p *imageStreamTagEvaluator) MatchingScopes(item runtime.Object, scopes []kapi.ScopedResourceSelectorRequirement) ([]kapi.ScopedResourceSelectorRequirement, error) {
	return []kapi.ScopedResourceSelectorRequirement{}, nil
}

func (p *imageStreamTagEvaluator) UncoveredQuotaScopes(limitedScopes []kapi.ScopedResourceSelectorRequirement, matchedQuotaScopes []kapi.ScopedResourceSelectorRequirement) ([]kapi.ScopedResourceSelectorRequirement, error) {
	return []kapi.ScopedResourceSelectorRequirement{}, nil
}

func (i *imageStreamTagEvaluator) MatchingResources(input []kapi.ResourceName) []kapi.ResourceName {
	return kquota.Intersection(input, imageStreamTagResources)
}

func (i *imageStreamTagEvaluator) Usage(item runtime.Object) (kapi.ResourceList, error) {
	if istInternal, ok := item.(*imageapi.ImageStreamTag); ok {
		out := &imagev1.ImageStreamTag{}
		if err := imagev1conversions.Convert_image_ImageStreamTag_To_v1_ImageStreamTag(istInternal, out, nil); err != nil {
			return kapi.ResourceList{}, fmt.Errorf("error converting ImageStreamImport: %v", err)
		}
		item = out
	}
	ist, ok := item.(*imagev1.ImageStreamTag)
	if !ok {
		return kapi.ResourceList{}, nil
	}

	res := map[kapi.ResourceName]resource.Quantity{
		imageapi.ResourceImageStreams: *resource.NewQuantity(0, resource.BinarySI),
	}

	isName, _, err := imageapi.ParseImageStreamTagName(ist.Name)
	if err != nil {
		return kapi.ResourceList{}, err
	}

	is, err := i.store.ImageStreams(ist.Namespace).Get(isName)
	if err != nil && !kerrors.IsNotFound(err) {
		utilruntime.HandleError(fmt.Errorf("failed to get image stream %s/%s: %v", ist.Namespace, isName, err))
	}
	if is == nil || kerrors.IsNotFound(err) {
		res[imageapi.ResourceImageStreams] = *resource.NewQuantity(1, resource.BinarySI)
	}

	return res, nil
}

func (i *imageStreamTagEvaluator) UsageStats(options kquota.UsageStatsOptions) (kquota.UsageStats, error) {
	return kquota.UsageStats{}, nil
}
