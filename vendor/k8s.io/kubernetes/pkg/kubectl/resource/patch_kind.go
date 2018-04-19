package resource

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var OAPIToGroupified func(uncast runtime.Object, gvk *schema.GroupVersionKind)

func fixOAPIGroupKind(uncast runtime.Object, gvk *schema.GroupVersionKind) {
	if OAPIToGroupified != nil {
		OAPIToGroupified(uncast, gvk)
	}
}
