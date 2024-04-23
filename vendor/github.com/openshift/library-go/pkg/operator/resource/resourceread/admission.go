package resourceread

import (
	admissionv1 "k8s.io/api/admissionregistration/v1"
	admissionv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	admissionScheme = runtime.NewScheme()
	admissionCodecs = serializer.NewCodecFactory(admissionScheme)
)

func init() {
	utilruntime.Must(admissionv1.AddToScheme(admissionScheme))
	utilruntime.Must(admissionv1beta1.AddToScheme(admissionScheme))
}

func ReadValidatingWebhookConfigurationV1OrDie(objBytes []byte) *admissionv1.ValidatingWebhookConfiguration {
	requiredObj, err := runtime.Decode(admissionCodecs.UniversalDecoder(admissionv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}

	return requiredObj.(*admissionv1.ValidatingWebhookConfiguration)
}

func ReadMutatingWebhookConfigurationV1OrDie(objBytes []byte) *admissionv1.MutatingWebhookConfiguration {
	requiredObj, err := runtime.Decode(admissionCodecs.UniversalDecoder(admissionv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}

	return requiredObj.(*admissionv1.MutatingWebhookConfiguration)
}

func ReadValidatingAdmissionPolicyV1beta1OrDie(objBytes []byte) *admissionv1beta1.ValidatingAdmissionPolicy {
	requiredObj, err := runtime.Decode(admissionCodecs.UniversalDecoder(admissionv1beta1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}

	return requiredObj.(*admissionv1beta1.ValidatingAdmissionPolicy)
}

func ReadValidatingAdmissionPolicyBindingV1beta1OrDie(objBytes []byte) *admissionv1beta1.ValidatingAdmissionPolicyBinding {
	requiredObj, err := runtime.Decode(admissionCodecs.UniversalDecoder(admissionv1beta1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}

	return requiredObj.(*admissionv1beta1.ValidatingAdmissionPolicyBinding)
}
