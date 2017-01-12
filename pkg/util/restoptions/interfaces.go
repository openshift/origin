package restoptions

import (
	genericrest "github.com/openshift/origin/pkg/util/restoptions/generic" // Temporary hack replacement for "k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

type Getter interface {
	GetRESTOptions(resource unversioned.GroupResource) (genericrest.RESTOptions, error)
}
