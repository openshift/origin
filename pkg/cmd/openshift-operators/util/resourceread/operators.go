package resourceread

import (
	webconsolev1alpha1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apis/webconsole/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	operatorsScheme = runtime.NewScheme()
	operatorsCodecs = serializer.NewCodecFactory(operatorsScheme)
)

func init() {
	if err := webconsolev1alpha1.AddToScheme(operatorsScheme); err != nil {
		panic(err)
	}
}

func ReadWebConsoleOperatorConfigOrDie(objBytes []byte) *webconsolev1alpha1.OpenShiftWebConsoleConfig {
	requiredObj, err := runtime.Decode(operatorsCodecs.UniversalDecoder(webconsolev1alpha1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*webconsolev1alpha1.OpenShiftWebConsoleConfig)
}
