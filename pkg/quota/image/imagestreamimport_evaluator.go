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

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinternalversion "github.com/openshift/origin/pkg/image/generated/listers/image/internalversion"
)

var imageStreamImportResources = []kapi.ResourceName{
	imageapi.ResourceImageStreams,
}

type imageStreamImportEvaluator struct {
	store imageinternalversion.ImageStreamLister
}

// NewImageStreamImportEvaluator computes resource usage for ImageStreamImport objects. This particular kind
// is a virtual resource. It depends on ImageStream usage evaluator to compute image numbers before the
// the admission can work.
func NewImageStreamImportEvaluator(store imageinternalversion.ImageStreamLister) kquota.Evaluator {
	return &imageStreamImportEvaluator{
		store: store,
	}
}

// Constraints checks that given object is an image stream import.
func (i *imageStreamImportEvaluator) Constraints(required []kapi.ResourceName, object runtime.Object) error {
	if _, ok := object.(*imageapi.ImageStreamImport); !ok {
		return fmt.Errorf("unexpected input object %v", object)
	}
	return nil
}

func (i *imageStreamImportEvaluator) GroupResource() schema.GroupResource {
	return imageapi.Resource("imagestreamimports")
}

func (i *imageStreamImportEvaluator) Handles(a kadmission.Attributes) bool {
	return a.GetOperation() == kadmission.Create
}

func (i *imageStreamImportEvaluator) Matches(resourceQuota *kapi.ResourceQuota, item runtime.Object) (bool, error) {
	matchesScopeFunc := func(kapi.ResourceQuotaScope, runtime.Object) (bool, error) { return true, nil }
	return generic.Matches(resourceQuota, item, i.MatchingResources, matchesScopeFunc)
}

func (i *imageStreamImportEvaluator) MatchingResources(input []kapi.ResourceName) []kapi.ResourceName {
	return kquota.Intersection(input, imageStreamImportResources)
}

func (i *imageStreamImportEvaluator) Usage(item runtime.Object) (kapi.ResourceList, error) {
	isi, ok := item.(*imageapi.ImageStreamImport)
	if !ok {
		return kapi.ResourceList{}, fmt.Errorf("item is not an ImageStreamImport: %T", item)
	}

	usage := map[kapi.ResourceName]resource.Quantity{
		imageapi.ResourceImageStreams: *resource.NewQuantity(0, resource.DecimalSI),
	}

	if !isi.Spec.Import || (len(isi.Spec.Images) == 0 && isi.Spec.Repository == nil) {
		return usage, nil
	}

	is, err := i.store.ImageStreams(isi.Namespace).Get(isi.Name)
	if err != nil && !kerrors.IsNotFound(err) {
		utilruntime.HandleError(fmt.Errorf("failed to list image streams: %v", err))
	}
	if is == nil || kerrors.IsNotFound(err) {
		usage[imageapi.ResourceImageStreams] = *resource.NewQuantity(1, resource.DecimalSI)
	}

	return usage, nil
}

func (i *imageStreamImportEvaluator) UsageStats(options kquota.UsageStatsOptions) (kquota.UsageStats, error) {
	return kquota.UsageStats{}, nil
}
