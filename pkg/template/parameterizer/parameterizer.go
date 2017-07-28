package parameterizer

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
)

// Parameterizer is an object that knows how to parameterize an aspect of objects in a
// template
type Parameterizer interface {
	// Parameterize will transform an object by setting a parameter value
	// Parameters include the object to parameterize
	Parameterize(obj runtime.Object, objs []runtime.Object, params Params) error
}

// Parameterize transforms a template by applying a set of Parameterizers to it
func Parameterize(template *templateapi.Template, parameterizers []Parameterizer) []error {
	params := ParamsFromList(template.Parameters)

	errs := []error{}
	decodedItems := []runtime.Object{}
	for _, item := range template.Objects {
		if obj, ok := item.(*runtime.Unknown); ok {
			decodedObj, err := runtime.Decode(kapi.Codecs.UniversalDecoder(), obj.Raw)
			if err != nil {
				errs = append(errs, fmt.Errorf("unable to handle object: %v", err))
				continue
			}
			decodedItems = append(decodedItems, decodedObj)
			continue
		}
		decodedItems = append(decodedItems, item)
	}
	if len(errs) > 0 {
		return errs
	}

	for i := range decodedItems {
		for _, parameterizer := range parameterizers {
			if err := parameterizer.Parameterize(decodedItems[i], decodedItems, params); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return errs
	}
	template.Objects = decodedItems
	template.Parameters = params.ToList()
	return nil
}
