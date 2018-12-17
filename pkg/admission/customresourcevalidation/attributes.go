package customresourcevalidation

import (
	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/admission"
)

// unstructuredUnpackingAttributes tries to convert to a real object in the config scheme
type unstructuredUnpackingAttributes struct {
	admission.Attributes
}

func (a *unstructuredUnpackingAttributes) GetObject() runtime.Object {
	return toBestObjectPossible(a.Attributes.GetObject())
}

func (a *unstructuredUnpackingAttributes) GetOldObject() runtime.Object {
	return toBestObjectPossible(a.Attributes.GetOldObject())
}

// toBestObjectPossible tries to convert to a real object in the config scheme
func toBestObjectPossible(orig runtime.Object) runtime.Object {
	unstructuredOrig, ok := orig.(runtime.Unstructured)
	if !ok {
		return orig
	}

	targetObj, err := configScheme.New(unstructuredOrig.GetObjectKind().GroupVersionKind())
	if err != nil {
		utilruntime.HandleError(err)
		return unstructuredOrig
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredOrig.UnstructuredContent(), targetObj); err != nil {
		utilruntime.HandleError(err)
		return unstructuredOrig
	}
	return targetObj
}

var configScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(configv1.Install(configScheme))
}
