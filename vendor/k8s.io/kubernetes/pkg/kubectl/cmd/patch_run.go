package cmd

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var OAPIToGroupifiedGVK func(gvk *schema.GroupVersionKind)

func FixOAPIGroupifiedGVK(gvk *schema.GroupVersionKind) {
	if OAPIToGroupifiedGVK != nil {
		OAPIToGroupifiedGVK(gvk)
	}
}

var UseOpenShiftGenerator = false
