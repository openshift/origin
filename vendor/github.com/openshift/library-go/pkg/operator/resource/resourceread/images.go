package resourceread

import (
	imagev1 "github.com/openshift/api/image/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	imagesScheme = runtime.NewScheme()
	imagesCodecs = serializer.NewCodecFactory(imagesScheme)
)

func init() {
	if err := imagev1.AddToScheme(imagesScheme); err != nil {
		panic(err)
	}
}

func ReadImageStreamV1OrDie(objBytes []byte) *imagev1.ImageStream {
	requiredObj, err := runtime.Decode(imagesCodecs.UniversalDecoder(imagev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*imagev1.ImageStream)
}
