package api

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
)

func ListOptionsToSelectors(options *metav1.ListOptions) (labels.Selector, fields.Selector, error) {
	label := ""
	if options != nil && options.LabelSelector != "" {
		label = options.LabelSelector
	}
	field := ""
	if options != nil && options.FieldSelector != "" {
		field = options.FieldSelector
	}
	labelSel, err := labels.Parse(label)
	if err != nil {
		return labels.Everything(), fields.Everything(), err
	}
	fieldSel, err := fields.ParseSelector(field)
	if err != nil {
		return labels.Everything(), fields.Everything(), err
	}
	return labelSel, fieldSel, nil
}

func InternalListOptionsToSelectors(options *metainternal.ListOptions) (labels.Selector, fields.Selector) {
	label := labels.Everything()
	if options != nil && options.LabelSelector != nil {
		label = options.LabelSelector
	}
	field := fields.Everything()
	if options != nil && options.FieldSelector != nil {
		field = options.FieldSelector
	}
	return label, field
}
