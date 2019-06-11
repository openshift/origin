package image

import (
	"fmt"

	"github.com/openshift/origin/pkg/quota/quotaimageexternal"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kadmission "k8s.io/apiserver/pkg/admission"
	kquota "k8s.io/kubernetes/pkg/quota/v1"

	imagev1 "github.com/openshift/api/image/v1"
	imagev1lister "github.com/openshift/client-go/image/listers/image/v1"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imagev1conversions "github.com/openshift/origin/pkg/image/apis/image/v1"
)

var imageStreamImportResources = []corev1.ResourceName{
	imagev1.ResourceImageStreams,
}

type imageStreamImportEvaluator struct {
	externalEvaluator kquota.Evaluator
}

// NewImageStreamImportEvaluator computes resource usage for ImageStreamImport objects. This particular kind
// is a virtual resource. It depends on ImageStream usage evaluator to compute image numbers before the
// the admission can work.
func NewImageStreamImportEvaluator(store imagev1lister.ImageStreamLister) kquota.Evaluator {
	return &imageStreamImportEvaluator{
		externalEvaluator: quotaimageexternal.NewImageStreamImportEvaluator(store),
	}
}

// Constraints checks that given object is an image stream import.
func (i *imageStreamImportEvaluator) Constraints(required []corev1.ResourceName, object runtime.Object) error {
	_, okInt := object.(*imageapi.ImageStreamImport)
	if okInt {
		return nil
	}
	return i.externalEvaluator.Constraints(required, object)
}

func (i *imageStreamImportEvaluator) GroupResource() schema.GroupResource {
	return i.externalEvaluator.GroupResource()
}

func (i *imageStreamImportEvaluator) Handles(a kadmission.Attributes) bool {
	return i.externalEvaluator.Handles(a)
}

func (i *imageStreamImportEvaluator) Matches(resourceQuota *corev1.ResourceQuota, item runtime.Object) (bool, error) {
	return i.externalEvaluator.Matches(resourceQuota, item)
}

func (i *imageStreamImportEvaluator) MatchingScopes(item runtime.Object, scopes []corev1.ScopedResourceSelectorRequirement) ([]corev1.ScopedResourceSelectorRequirement, error) {
	return i.externalEvaluator.MatchingScopes(item, scopes)
}

func (i *imageStreamImportEvaluator) UncoveredQuotaScopes(limitedScopes []corev1.ScopedResourceSelectorRequirement, matchedQuotaScopes []corev1.ScopedResourceSelectorRequirement) ([]corev1.ScopedResourceSelectorRequirement, error) {
	return i.externalEvaluator.UncoveredQuotaScopes(limitedScopes, matchedQuotaScopes)
}

func (i *imageStreamImportEvaluator) MatchingResources(input []corev1.ResourceName) []corev1.ResourceName {
	return i.externalEvaluator.MatchingResources(input)
}

func (i *imageStreamImportEvaluator) Usage(item runtime.Object) (corev1.ResourceList, error) {
	if isiInternal, ok := item.(*imageapi.ImageStreamImport); ok {
		out := &imagev1.ImageStreamImport{}
		if err := imagev1conversions.Convert_image_ImageStreamImport_To_v1_ImageStreamImport(isiInternal, out, nil); err != nil {
			return corev1.ResourceList{}, fmt.Errorf("error converting ImageStreamImport: %v", err)
		}
		item = out
	}
	return i.externalEvaluator.Usage(item)
}

func (i *imageStreamImportEvaluator) UsageStats(options kquota.UsageStatsOptions) (kquota.UsageStats, error) {
	return i.externalEvaluator.UsageStats(options)
}
