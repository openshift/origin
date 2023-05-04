package resourceread

import (
	"github.com/openshift/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	migrationv1alpha1 "sigs.k8s.io/kube-storage-version-migrator/pkg/apis/migration/v1alpha1"
)

var (
	genericScheme = runtime.NewScheme()
	genericCodecs = serializer.NewCodecFactory(genericScheme)
	genericCodec  = genericCodecs.UniversalDeserializer()
)

func init() {
	utilruntime.Must(api.Install(genericScheme))
	utilruntime.Must(api.InstallKube(genericScheme))
	utilruntime.Must(apiextensionsv1beta1.AddToScheme(genericScheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(genericScheme))
	utilruntime.Must(migrationv1alpha1.AddToScheme(genericScheme))
	utilruntime.Must(admissionregistrationv1.AddToScheme(genericScheme))
}

// ReadGenericWithUnstructured parses given yaml file using known scheme (see genericScheme above).
// If the object kind is not registered in the scheme, it returns Unstructured as the last resort.
func ReadGenericWithUnstructured(objBytes []byte) (runtime.Object, error) {
	// Try to get a typed object first
	typedObj, _, decodeErr := genericCodec.Decode(objBytes, nil, nil)
	if decodeErr == nil {
		return typedObj, nil
	}

	// Try unstructured, hoping to recover from "no kind XXX is registered for version YYY"
	unstructuredObj, _, err := scheme.Codecs.UniversalDecoder().Decode(objBytes, nil, &unstructured.Unstructured{})
	if err != nil {
		// Return the original error
		return nil, decodeErr
	}
	return unstructuredObj, nil
}

// ReadGenericWithUnstructuredOrDie parses given yaml file using known scheme (see genericScheme above).
// If the object kind is not registered in the scheme, it returns Unstructured as the last resort.
func ReadGenericWithUnstructuredOrDie(objBytes []byte) runtime.Object {
	obj, err := ReadGenericWithUnstructured(objBytes)
	if err != nil {
		panic(err)
	}
	return obj
}
