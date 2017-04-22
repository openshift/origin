package restoptions

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"
)

type Getter interface {
	GetRESTOptions(resource schema.GroupResource) (generic.RESTOptions, error)
}
