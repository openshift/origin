package factory

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
)

func ObjectNameToKey(obj runtime.Object) string {
	metaObj, ok := obj.(metav1.ObjectMetaAccessor)
	if !ok {
		return ""
	}
	return metaObj.GetObjectMeta().GetName()
}

func NamesFilter(names ...string) EventFilterFunc {
	nameSet := sets.NewString(names...)
	return func(obj interface{}) bool {
		metaObj, ok := obj.(metav1.ObjectMetaAccessor)
		if !ok {
			return false
		}
		return nameSet.Has(metaObj.GetObjectMeta().GetName())
	}
}
