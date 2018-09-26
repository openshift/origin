package resourceread

import (
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	apiExtensionsScheme = runtime.NewScheme()
	apiExtensionsCodecs = serializer.NewCodecFactory(apiExtensionsScheme)
)

func init() {
	if err := apiextensionsv1beta1.AddToScheme(apiExtensionsScheme); err != nil {
		panic(err)
	}
}

func ReadCustomResourceDefinitionV1Beta1OrDie(objBytes []byte) *apiextensionsv1beta1.CustomResourceDefinition {
	requiredObj, err := runtime.Decode(apiExtensionsCodecs.UniversalDecoder(apiextensionsv1beta1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*apiextensionsv1beta1.CustomResourceDefinition)
}
