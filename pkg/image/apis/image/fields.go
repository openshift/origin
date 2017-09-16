package image

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func ImageStreamSelector(obj runtime.Object, fieldSet fields.Set) error {
	imageStream, ok := obj.(*ImageStream)
	if !ok {
		return fmt.Errorf("%T not an ImageStream", obj)
	}
	fieldSet["spec.dockerImageRepository"] = imageStream.Spec.DockerImageRepository
	fieldSet["status.dockerImageRepository"] = imageStream.Status.DockerImageRepository

	return nil
}
