package resourceread

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
)

func ReadConfigMapOrDie(objBytes []byte) *corev1.ConfigMap {
	requiredObj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(corev1.SchemeGroupVersion), []byte(objBytes))
	if err != nil {
		panic(err)
	}
	return requiredObj.(*corev1.ConfigMap)
}

func ReadServiceOrDie(objBytes []byte) *corev1.Service {
	requiredObj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(corev1.SchemeGroupVersion), []byte(objBytes))
	if err != nil {
		panic(err)
	}
	return requiredObj.(*corev1.Service)
}

func ReadNamespaceOrDie(objBytes []byte) *corev1.Namespace {
	requiredObj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(corev1.SchemeGroupVersion), []byte(objBytes))
	if err != nil {
		panic(err)
	}
	return requiredObj.(*corev1.Namespace)
}
