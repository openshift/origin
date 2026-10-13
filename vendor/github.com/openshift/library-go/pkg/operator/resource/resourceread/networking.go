package resourceread

import (
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	netScheme = runtime.NewScheme()
	netCodecs = serializer.NewCodecFactory(netScheme)
)

func init() {
	if err := networkingv1.AddToScheme(netScheme); err != nil {
		panic(err)
	}
}

func ReadNetworkPolicyV1OrDie(objBytes []byte) *networkingv1.NetworkPolicy {
	requiredObj, err := runtime.Decode(netCodecs.UniversalDecoder(networkingv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*networkingv1.NetworkPolicy)
}
