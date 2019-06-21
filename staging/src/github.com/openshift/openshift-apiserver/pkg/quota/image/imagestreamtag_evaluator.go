package image

import (
	"fmt"

	"github.com/openshift/openshift-apiserver/pkg/quota/quotaimageexternal"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kadmission "k8s.io/apiserver/pkg/admission"
	kquota "k8s.io/kubernetes/pkg/quota/v1"

	imagev1 "github.com/openshift/api/image/v1"
	imagev1typedclient "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	imagev1lister "github.com/openshift/client-go/image/listers/image/v1"
	imageapi "github.com/openshift/openshift-apiserver/pkg/image/apis/image"
	imagev1conversions "github.com/openshift/openshift-apiserver/pkg/image/apis/image/v1"
)

type imageStreamTagEvaluator struct {
	externalEvaluator kquota.Evaluator
}

// NewImageStreamTagEvaluator computes resource usage of ImageStreamsTags. Its sole purpose is to handle
// UPDATE admission operations on imageStreamTags resource.
func NewImageStreamTagEvaluator(store imagev1lister.ImageStreamLister, istGetter imagev1typedclient.ImageStreamTagsGetter) kquota.Evaluator {
	return &imageStreamTagEvaluator{
		externalEvaluator: quotaimageexternal.NewImageStreamTagEvaluator(store, istGetter),
	}
}

// Constraints checks that given object is an image stream tag
func (i *imageStreamTagEvaluator) Constraints(required []corev1.ResourceName, object runtime.Object) error {
	_, okInt := object.(*imageapi.ImageStreamTag)
	if okInt {
		return nil
	}
	return i.externalEvaluator.Constraints(required, object)
}

func (i *imageStreamTagEvaluator) GroupResource() schema.GroupResource {
	return i.externalEvaluator.GroupResource()
}

func (i *imageStreamTagEvaluator) Handles(a kadmission.Attributes) bool {
	return i.externalEvaluator.Handles(a)
}

func (i *imageStreamTagEvaluator) Matches(resourceQuota *corev1.ResourceQuota, item runtime.Object) (bool, error) {
	return i.externalEvaluator.Matches(resourceQuota, item)
}

func (i *imageStreamTagEvaluator) MatchingScopes(item runtime.Object, scopes []corev1.ScopedResourceSelectorRequirement) ([]corev1.ScopedResourceSelectorRequirement, error) {
	return i.externalEvaluator.MatchingScopes(item, scopes)
}

func (i *imageStreamTagEvaluator) UncoveredQuotaScopes(limitedScopes []corev1.ScopedResourceSelectorRequirement, matchedQuotaScopes []corev1.ScopedResourceSelectorRequirement) ([]corev1.ScopedResourceSelectorRequirement, error) {
	return i.externalEvaluator.UncoveredQuotaScopes(limitedScopes, matchedQuotaScopes)
}

func (i *imageStreamTagEvaluator) MatchingResources(input []corev1.ResourceName) []corev1.ResourceName {
	return i.externalEvaluator.MatchingResources(input)
}

func (i *imageStreamTagEvaluator) Usage(item runtime.Object) (corev1.ResourceList, error) {
	if istInternal, ok := item.(*imageapi.ImageStreamTag); ok {
		out := &imagev1.ImageStreamTag{}
		if err := imagev1conversions.Convert_image_ImageStreamTag_To_v1_ImageStreamTag(istInternal, out, nil); err != nil {
			return corev1.ResourceList{}, fmt.Errorf("error converting ImageStreamImport: %v", err)
		}
		item = out
	}
	return i.externalEvaluator.Usage(item)
}

func (i *imageStreamTagEvaluator) UsageStats(options kquota.UsageStatsOptions) (kquota.UsageStats, error) {
	return i.externalEvaluator.UsageStats(options)
}
