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
	imagev1typedclient "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	imagev1lister "github.com/openshift/client-go/image/listers/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
)

var imageStreamTagResources = []corev1.ResourceName{
	imagev1.ResourceImageStreams,
}

type imageStreamTagEvaluator struct {
	store     imagev1lister.ImageStreamLister
	istGetter imagev1typedclient.ImageStreamTagsGetter
}

// NewImageStreamTagEvaluator computes resource usage of ImageStreamsTags. Its sole purpose is to handle
// UPDATE admission operations on imageStreamTags resource.
// If you make a change here, be sure to make a corresponding change in the apiserver
func NewImageStreamTagEvaluator(store imagev1lister.ImageStreamLister, istGetter imagev1typedclient.ImageStreamTagsGetter) kquota.Evaluator {
	return &imageStreamTagEvaluator{
		store:     store,
		istGetter: istGetter,
	}
}

// Constraints checks that given object is an image stream tag
func (i *imageStreamTagEvaluator) Constraints(required []corev1.ResourceName, object runtime.Object) error {
	_, okExt := object.(*imagev1.ImageStreamTag)
	if !okExt {
		return fmt.Errorf("unexpected input object %v", object)
	}
	return nil
}

func (i *imageStreamTagEvaluator) GroupResource() schema.GroupResource {
	return schema.GroupResource{Group: "image.openshift.io", Resource: "imagestreamtags"}
}

func (i *imageStreamTagEvaluator) Handles(a kadmission.Attributes) bool {
	operation := a.GetOperation()
	return operation == kadmission.Create || operation == kadmission.Update
}

func (i *imageStreamTagEvaluator) Matches(resourceQuota *corev1.ResourceQuota, item runtime.Object) (bool, error) {
	matchesScopeFunc := func(corev1.ScopedResourceSelectorRequirement, runtime.Object) (bool, error) { return true, nil }
	return generic.Matches(resourceQuota, item, i.MatchingResources, matchesScopeFunc)
}

func (p *imageStreamTagEvaluator) MatchingScopes(item runtime.Object, scopes []corev1.ScopedResourceSelectorRequirement) ([]corev1.ScopedResourceSelectorRequirement, error) {
	return []corev1.ScopedResourceSelectorRequirement{}, nil
}

func (p *imageStreamTagEvaluator) UncoveredQuotaScopes(limitedScopes []corev1.ScopedResourceSelectorRequirement, matchedQuotaScopes []corev1.ScopedResourceSelectorRequirement) ([]corev1.ScopedResourceSelectorRequirement, error) {
	return []corev1.ScopedResourceSelectorRequirement{}, nil
}

func (i *imageStreamTagEvaluator) MatchingResources(input []corev1.ResourceName) []corev1.ResourceName {
	return kquota.Intersection(input, imageStreamTagResources)
}

func (i *imageStreamTagEvaluator) Usage(item runtime.Object) (corev1.ResourceList, error) {
	ist, ok := item.(*imagev1.ImageStreamTag)
	if !ok {
		return corev1.ResourceList{}, nil
	}

	res := map[corev1.ResourceName]resource.Quantity{
		imagev1.ResourceImageStreams: *resource.NewQuantity(0, resource.BinarySI),
	}

	isName, _, err := imageutil.ParseImageStreamTagName(ist.Name)
	if err != nil {
		return corev1.ResourceList{}, err
	}

	is, err := i.store.ImageStreams(ist.Namespace).Get(isName)
	if err != nil && !kerrors.IsNotFound(err) {
		utilruntime.HandleError(fmt.Errorf("failed to get image stream %s/%s: %v", ist.Namespace, isName, err))
	}
	if is == nil || kerrors.IsNotFound(err) {
		res[imagev1.ResourceImageStreams] = *resource.NewQuantity(1, resource.BinarySI)
	}

	return res, nil
}

func (i *imageStreamTagEvaluator) UsageStats(options kquota.UsageStatsOptions) (kquota.UsageStats, error) {
	return kquota.UsageStats{}, nil
}
