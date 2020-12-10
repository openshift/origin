package resourceread

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	apiExtensionsScheme = runtime.NewScheme()
	apiExtensionsCodecs = serializer.NewCodecFactory(apiExtensionsScheme)
)

func init() {
	utilruntime.Must(apiextensionsv1beta1.AddToScheme(apiExtensionsScheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(apiExtensionsScheme))
}

func ReadCustomResourceDefinitionV1Beta1OrDie(objBytes []byte) *apiextensionsv1beta1.CustomResourceDefinition {
	requiredObj, err := runtime.Decode(apiExtensionsCodecs.UniversalDecoder(apiextensionsv1beta1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*apiextensionsv1beta1.CustomResourceDefinition)
}

func ReadCustomResourceDefinitionV1OrDie(objBytes []byte) *apiextensionsv1.CustomResourceDefinition {
	requiredObj, err := runtime.Decode(apiExtensionsCodecs.UniversalDecoder(apiextensionsv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*apiextensionsv1.CustomResourceDefinition)
}
