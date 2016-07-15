package api

import (
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
)

var accessor = meta.NewAccessor()

func GetMatcher(selector ClusterResourceQuotaSelector) (func(obj runtime.Object) (bool, error), error) {
	var labelSelector labels.Selector
	if selector.LabelSelector != nil {
		var err error
		labelSelector, err = unversioned.LabelSelectorAsSelector(selector.LabelSelector)
		if err != nil {
			return nil, err
		}
	}

	var annotationSelector labels.Selector
	if len(selector.AnnotationSelector) > 0 {
		var err error
		annotationSelector, err = unversioned.LabelSelectorAsSelector(&unversioned.LabelSelector{MatchLabels: selector.AnnotationSelector})
		if err != nil {
			return nil, err
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
			if !annotationSelector.Matches(labels.Set(objAnnotations)) {
				return false, nil
			}
		}

		return true, nil
	}, nil
}
