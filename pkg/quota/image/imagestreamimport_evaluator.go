package image

import (
	"fmt"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/resource"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/generic"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"

	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const imageStreamImportName = "Evaluator.ImageStreamImport"

// NewImageStreamImportEvaluator computes resource usage for ImageStreamImport objects. This particular kind
// is a virtual resource. It depends on ImageStream usage evaluator to compute image numbers before the
// the admission can work.
func NewImageStreamImportEvaluator(isNamespacer osclient.ImageStreamsNamespacer) kquota.Evaluator {
	computeResources := []kapi.ResourceName{
		imageapi.ResourceImageStreams,
	}

	matchesScopeFunc := func(kapi.ResourceQuotaScope, runtime.Object) bool { return true }

	return &generic.GenericEvaluator{
		Name:                       imageStreamImportName,
		InternalGroupKind:          imageapi.Kind("ImageStreamImport"),
		InternalOperationResources: map[admission.Operation][]kapi.ResourceName{admission.Create: computeResources},
		MatchedResourceNames:       computeResources,
		MatchesScopeFunc:           matchesScopeFunc,
		UsageFunc:                  makeImageStreamImportAdmissionUsageFunc(isNamespacer),
		ListFuncByNamespace: func(namespace string, options kapi.ListOptions) (runtime.Object, error) {
			return &kapi.List{}, nil
		},
		ConstraintsFunc: imageStreamImportConstraintsFunc,
	}
}

// imageStreamImportConstraintsFunc checks that given object is an image stream import.
func imageStreamImportConstraintsFunc(required []kapi.ResourceName, object runtime.Object) error {
	if _, ok := object.(*imageapi.ImageStreamImport); !ok {
		return fmt.Errorf("unexpected input object %v", object)
	}
	return nil
}

// makeImageStreamImportAdmissionUsageFunc retuns a function for computing a usage of an image stream import.
func makeImageStreamImportAdmissionUsageFunc(isNamespacer osclient.ImageStreamsNamespacer) generic.UsageFunc {
	return func(object runtime.Object) kapi.ResourceList {
		isi, ok := object.(*imageapi.ImageStreamImport)
		if !ok {
			return kapi.ResourceList{}
		}

		usage := map[kapi.ResourceName]resource.Quantity{
			imageapi.ResourceImageStreams: *resource.NewQuantity(0, resource.DecimalSI),
		}

		if !isi.Spec.Import || (len(isi.Spec.Images) == 0 && isi.Spec.Repository == nil) {
			return usage
		}

		is, err := isNamespacer.ImageStreams(isi.Namespace).Get(isi.Name)
		if err != nil && !kerrors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("failed to list image streams: %v", err))
		}
		if is == nil || kerrors.IsNotFound(err) {
			usage[imageapi.ResourceImageStreams] = *resource.NewQuantity(1, resource.DecimalSI)
		}

		return usage
	}
}
