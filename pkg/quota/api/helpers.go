package api

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/labels"
)

var accessor = meta.NewAccessor()

func GetMatcher(selector ClusterResourceQuotaSelector) (func(obj runtime.Object) (bool, error), error) {
	var labelSelector labels.Selector
	if selector.LabelSelector != nil {
		var err error
		labelSelector, err = metav1.LabelSelectorAsSelector(selector.LabelSelector)
		if err != nil {
			return nil, err
		}
	}

	var annotationSelector map[string]string
	if len(selector.AnnotationSelector) > 0 {
		// ensure our matcher has a stable copy of the map
		annotationSelector = make(map[string]string, len(selector.AnnotationSelector))
		for k, v := range selector.AnnotationSelector {
			annotationSelector[k] = v
		}
	}

	return func(obj runtime.Object) (bool, error) {
		if labelSelector != nil {
			objLabels, err := accessor.Labels(obj)
			if err != nil {
				return false, err
			}
			if !labelSelector.Matches(labels.Set(objLabels)) {
				return false, nil
			}
		}

		if annotationSelector != nil {
			objAnnotations, err := accessor.Annotations(obj)
			if err != nil {
				return false, err
			}
			for k, v := range annotationSelector {
				if objValue, exists := objAnnotations[k]; !exists || objValue != v {
					return false, nil
				}
			}
		}

		return true, nil
	}, nil
}
