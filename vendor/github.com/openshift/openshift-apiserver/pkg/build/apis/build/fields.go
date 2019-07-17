package build

import (
	"fmt"

	"github.com/openshift/library-go/pkg/build/naming"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
)

func BuildFieldSelector(obj runtime.Object, fieldSet fields.Set) error {
	build, ok := obj.(*Build)
	if !ok {
		return fmt.Errorf("%T not a Build", obj)
	}
	fieldSet["status"] = string(build.Status.Phase)
	fieldSet["podName"] = naming.GetPodName(build.Name, "build")

	return nil
}
