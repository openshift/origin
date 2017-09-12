package build

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func BuildFieldSelector(obj runtime.Object, fieldSet fields.Set) error {
	build, ok := obj.(*Build)
	if !ok {
		return fmt.Errorf("%T not a Build", obj)
	}
	fieldSet["status"] = string(build.Status.Phase)
	fieldSet["podName"] = GetBuildPodName(build)

	return nil
}
