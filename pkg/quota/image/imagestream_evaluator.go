package image

import (
	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/generic"
	"k8s.io/kubernetes/pkg/runtime"

	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const imageStreamEvaluatorName = "Evaluator.ImageStream"

// NewImageStreamEvaluator computes resource usage of ImageStreams. Instantiating this is necessary for
// resource quota admission controller to properly work on image stream related objects.
func NewImageStreamEvaluator(isNamespacer osclient.ImageStreamsNamespacer) kquota.Evaluator {
	allResources := []kapi.ResourceName{
		imageapi.ResourceImageStreams,
	}

	return &generic.GenericEvaluator{
		Name:              imageStreamEvaluatorName,
		InternalGroupKind: imageapi.Kind("ImageStream"),
		InternalOperationResources: map[admission.Operation][]kapi.ResourceName{
			admission.Create: allResources,
		},
		MatchedResourceNames: allResources,
		MatchesScopeFunc:     generic.MatchesNoScopeFunc,
		ConstraintsFunc:      generic.ObjectCountConstraintsFunc(imageapi.ResourceImageStreams),
		UsageFunc:            generic.ObjectCountUsageFunc(imageapi.ResourceImageStreams),
		ListFuncByNamespace: func(namespace string, options kapi.ListOptions) (runtime.Object, error) {
			return isNamespacer.ImageStreams(namespace).List(options)
		},
	}
}
