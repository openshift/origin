package image

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/generic"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinternalversion "github.com/openshift/origin/pkg/image/generated/listers/image/internalversion"
)

// NewImageStreamEvaluator computes resource usage of ImageStreams. Instantiating this is necessary for
// resource quota admission controller to properly work on image stream related objects.
func NewImageStreamEvaluator(store imageinternalversion.ImageStreamLister) kquota.Evaluator {
	return &generic.ObjectCountEvaluator{
		AllowCreateOnUpdate: false,
		InternalGroupKind:   imageapi.Kind("ImageStream"),
		ResourceName:        imageapi.ResourceImageStreams,
		ListFuncByNamespace: func(namespace string, options metav1.ListOptions) ([]runtime.Object, error) {
			labelSelector, err := labels.Parse(options.LabelSelector)
			if err != nil {
				return nil, err
			}
			list, err := store.ImageStreams(namespace).List(labelSelector)
			if err != nil {
				return nil, err
			}
			results := make([]runtime.Object, 0, len(list))
			for _, is := range list {
				results = append(results, is)
			}
			return results, nil
		},
	}
}
