package resourceread

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
)

func ReadDeploymentOrDie(objBytes []byte) *appsv1.Deployment {
	requiredObj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(appsv1.SchemeGroupVersion), []byte(objBytes))
	if err != nil {
		panic(err)
	}
	return requiredObj.(*appsv1.Deployment)
}

func ReadDaemonSetOrDie(objBytes []byte) *appsv1.DaemonSet {
	requiredObj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(appsv1.SchemeGroupVersion), []byte(objBytes))
	if err != nil {
		panic(err)
	}
	return requiredObj.(*appsv1.DaemonSet)
}
