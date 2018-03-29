package resourceread

import (
	apiserverv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/apis/apiserver/v1"
	controllerv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/apis/controller/v1"
	webconsolev1 "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/apis/webconsole/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	operatorsScheme = runtime.NewScheme()
	operatorsCodecs = serializer.NewCodecFactory(operatorsScheme)
)

func init() {
	if err := apiserverv1.AddToScheme(operatorsScheme); err != nil {
		panic(err)
	}
	if err := controllerv1.AddToScheme(operatorsScheme); err != nil {
		panic(err)
	}
	if err := webconsolev1.AddToScheme(operatorsScheme); err != nil {
		panic(err)
	}
}

func ReadAPIServerOperatorConfigOrDie(objBytes []byte) *apiserverv1.OpenShiftAPIServerConfig {
	requiredObj, err := runtime.Decode(operatorsCodecs.UniversalDecoder(apiserverv1.SchemeGroupVersion), []byte(objBytes))
	if err != nil {
		panic(err)
	}
	return requiredObj.(*apiserverv1.OpenShiftAPIServerConfig)
}
func ReadControllerOperatorConfigOrDie(objBytes []byte) *controllerv1.OpenShiftControllerConfig {
	requiredObj, err := runtime.Decode(operatorsCodecs.UniversalDecoder(controllerv1.SchemeGroupVersion), []byte(objBytes))
	if err != nil {
		panic(err)
	}
	return requiredObj.(*controllerv1.OpenShiftControllerConfig)
}

func ReadWebConsoleOperatorConfigOrDie(objBytes []byte) *webconsolev1.OpenShiftWebConsoleConfig {
	requiredObj, err := runtime.Decode(operatorsCodecs.UniversalDecoder(webconsolev1.SchemeGroupVersion), []byte(objBytes))
	if err != nil {
		panic(err)
	}
	return requiredObj.(*webconsolev1.OpenShiftWebConsoleConfig)
}
