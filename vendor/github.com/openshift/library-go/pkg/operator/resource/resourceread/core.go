package resourceread

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	coreScheme = runtime.NewScheme()
	coreCodecs = serializer.NewCodecFactory(coreScheme)
)

func init() {
	if err := corev1.AddToScheme(coreScheme); err != nil {
		panic(err)
	}
}

func ReadConfigMapV1OrDie(objBytes []byte) *corev1.ConfigMap {
	requiredObj, err := runtime.Decode(coreCodecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*corev1.ConfigMap)
}

func ReadSecretV1OrDie(objBytes []byte) *corev1.Secret {
	requiredObj, err := runtime.Decode(coreCodecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*corev1.Secret)
}

func ReadNamespaceV1OrDie(objBytes []byte) *corev1.Namespace {
	requiredObj, err := runtime.Decode(coreCodecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*corev1.Namespace)
}

func ReadServiceAccountV1OrDie(objBytes []byte) *corev1.ServiceAccount {
	requiredObj, err := runtime.Decode(coreCodecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*corev1.ServiceAccount)
}

func ReadServiceV1OrDie(objBytes []byte) *corev1.Service {
	requiredObj, err := runtime.Decode(coreCodecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*corev1.Service)
}

func ReadPodV1OrDie(objBytes []byte) *corev1.Pod {
	requiredObj, err := ReadPodV1(objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj
}

func ReadPodV1(objBytes []byte) (*corev1.Pod, error) {
	requiredObj, err := runtime.Decode(coreCodecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		return nil, err
	}
	return requiredObj.(*corev1.Pod), nil
}

func WritePodV1OrDie(obj *corev1.Pod) string {
	return runtime.EncodeOrDie(coreCodecs.LegacyCodec(corev1.SchemeGroupVersion), obj)
}
