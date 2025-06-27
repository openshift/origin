package resourceread

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

var (
	apiRegisterScheme = runtime.NewScheme()
	apiRegisterCodec  = serializer.NewCodecFactory(apiRegisterScheme)
)

func init() {
	if err := apiregistrationv1.AddToScheme(apiRegisterScheme); err != nil {
		panic(err)
	}
}

func ReadAPIServiceOrDie(objBytes []byte) *apiregistrationv1.APIService {
	requiredObj, err := runtime.Decode(apiRegisterCodec.UniversalDecoder(apiregistrationv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*apiregistrationv1.APIService)
}
