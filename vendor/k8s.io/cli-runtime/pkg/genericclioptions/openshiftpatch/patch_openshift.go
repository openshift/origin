package openshiftpatch

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var IsOC = false

var OAPIToGroupifiedGVK func(gvk *schema.GroupVersionKind)

func FixOAPIGroupifiedGVK(gvk *schema.GroupVersionKind) {
	if OAPIToGroupifiedGVK != nil {
		OAPIToGroupifiedGVK(gvk)
	}
}

var OAPIToGroupified func(uncast runtime.Object, gvk *schema.GroupVersionKind)

func FixOAPIGroupKind(uncast runtime.Object, gvk *schema.GroupVersionKind) {
	if OAPIToGroupified != nil {
		OAPIToGroupified(uncast, gvk)
	}
}

var IsOAPIFn func(gvk schema.GroupVersionKind) bool

func IsOAPI(gvk schema.GroupVersionKind) bool {
	if IsOAPIFn == nil {
		return false
	}

	return IsOAPIFn(gvk)
}
