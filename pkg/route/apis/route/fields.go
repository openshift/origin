package route

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func RouteFieldSelector(obj runtime.Object, fieldSet fields.Set) error {
	route, ok := obj.(*Route)
	if !ok {
		return fmt.Errorf("%T not a Route", obj)
	}
	fieldSet["spec.path"] = route.Spec.Path
	fieldSet["spec.host"] = route.Spec.Host
	fieldSet["spec.to.name"] = route.Spec.To.Name
	return nil
}
