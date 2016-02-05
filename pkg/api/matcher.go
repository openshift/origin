package api

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
)

func ListOptionsToSelectors(options *unversioned.ListOptions) (labels.Selector, fields.Selector) {
	label := labels.Everything()
	if options != nil && options.LabelSelector.Selector != nil {
		label = options.LabelSelector.Selector
	}
	field := fields.Everything()
	if options != nil && options.FieldSelector.Selector != nil {
		field = options.FieldSelector.Selector
	}
	return label, field
}
