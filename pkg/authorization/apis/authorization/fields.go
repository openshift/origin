package authorization

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func PolicyBindingFieldSelector(obj runtime.Object, fieldSet fields.Set) error {
	policyBinding, ok := obj.(*PolicyBinding)
	if !ok {
		return fmt.Errorf("%T not a PolicyBinding", obj)
	}
	fieldSet["policyRef.namespace"] = policyBinding.PolicyRef.Namespace
	return nil
}
