package resourceread

import (
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	policyScheme = runtime.NewScheme()
	policyCodecs = serializer.NewCodecFactory(policyScheme)
)

func init() {
	utilruntime.Must(policyv1.AddToScheme(policyScheme))
}

func ReadPodDisruptionBudgetV1OrDie(objBytes []byte) *policyv1.PodDisruptionBudget {
	requiredObj, err := runtime.Decode(policyCodecs.UniversalDecoder(policyv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*policyv1.PodDisruptionBudget)
}
