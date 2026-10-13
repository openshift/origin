package resourceread

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
)

func ReadCredentialRequestsOrDie(objBytes []byte) *unstructured.Unstructured {
	return ReadUnstructuredOrDie(objBytes)
}

func ReadUnstructuredOrDie(objBytes []byte) *unstructured.Unstructured {
	udi, _, err := scheme.Codecs.UniversalDecoder().Decode(objBytes, nil, &unstructured.Unstructured{})
	if err != nil {
		panic(err)
	}
	return udi.(*unstructured.Unstructured)
}
