package quotaimageexternal

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kadmission "k8s.io/apiserver/pkg/admission"
	kquota "k8s.io/kubernetes/pkg/quota/v1"
	"k8s.io/kubernetes/pkg/quota/v1/generic"

	imagev1 "github.com/openshift/api/image/v1"
	imagev1lister "github.com/openshift/client-go/image/listers/image/v1"
)

var imageStreamImportResources = []corev1.ResourceName{
	imagev1.ResourceImageStreams,
}

type imageStreamImportEvaluator struct {
	store imagev1lister.ImageStreamLister
}

// NewImageStreamImportEvaluator computes resource usage for ImageStreamImport objects. This particular kind
// is a virtual resource. It depends on ImageStream usage evaluator to compute image numbers before the
// the admission can work.
// If you make a change here, be sure to make a corresponding change in the apiserver
func NewImageStreamImportEvaluator(store imagev1lister.ImageStreamLister) kquota.Evaluator {
	return &imageStreamImportEvaluator{
		store: store,
	}
}

// Constraints checks that given object is an image stream import.
func (i *imageStreamImportEvaluator) Constraints(required []corev1.ResourceName, object runtime.Object) error {
	_, okExt := object.(*imagev1.ImageStreamImport)
	if !okExt {
		return fmt.Errorf("unexpected input object %v", object)
	}
	return nil
}

func (i *imageStreamImportEvaluator) GroupResource() schema.GroupResource {
	return schema.GroupResource{Group: "image.openshift.io", Resource: "imagestreamimports"}
}

func (i *imageStreamImportEvaluator) Handles(a kadmission.Attributes) bool {
	return a.GetOperation() == kadmission.Create
}

func (i *imageStreamImportEvaluator) Matches(resourceQuota *corev1.ResourceQuota, item runtime.Object) (bool, error) {
	matchesScopeFunc := func(corev1.ScopedResourceSelectorRequirement, runtime.Object) (bool, error) { return true, nil }
	return generic.Matches(resourceQuota, item, i.MatchingResources, matchesScopeFunc)
}

func (p *imageStreamImportEvaluator) MatchingScopes(item runtime.Object, scopes []corev1.ScopedResourceSelectorRequirement) ([]corev1.ScopedResourceSelectorRequirement, error) {
	return []corev1.ScopedResourceSelectorRequirement{}, nil
}

func (p *imageStreamImportEvaluator) UncoveredQuotaScopes(limitedScopes []corev1.ScopedResourceSelectorRequirement, matchedQuotaScopes []corev1.ScopedResourceSelectorRequirement) ([]corev1.ScopedResourceSelectorRequirement, error) {
	return []corev1.ScopedResourceSelectorRequirement{}, nil
}

func (i *imageStreamImportEvaluator) MatchingResources(input []corev1.ResourceName) []corev1.ResourceName {
	return kquota.Intersection(input, imageStreamImportResources)
}

func (i *imageStreamImportEvaluator) Usage(item runtime.Object) (corev1.ResourceList, error) {
	isi, ok := item.(*imagev1.ImageStreamImport)
	if !ok {
		return corev1.ResourceList{}, fmt.Errorf("item is not an ImageStreamImport: %T", item)
	}

	usage := map[corev1.ResourceName]resource.Quantity{
		imagev1.ResourceImageStreams: *resource.NewQuantity(0, resource.DecimalSI),
	}

	if !isi.Spec.Import || (len(isi.Spec.Images) == 0 && isi.Spec.Repository == nil) {
		return usage, nil
	}

	is, err := i.store.ImageStreams(isi.Namespace).Get(isi.Name)
	if err != nil && !kerrors.IsNotFound(err) {
		utilruntime.HandleError(fmt.Errorf("failed to list image streams: %v", err))
	}
	if is == nil || kerrors.IsNotFound(err) {
		usage[imagev1.ResourceImageStreams] = *resource.NewQuantity(1, resource.DecimalSI)
	}

	return usage, nil
}

func (i *imageStreamImportEvaluator) UsageStats(options kquota.UsageStatsOptions) (kquota.UsageStats, error) {
	return kquota.UsageStats{}, nil
}
