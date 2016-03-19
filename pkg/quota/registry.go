package quota

import (
	kquota "k8s.io/kubernetes/pkg/quota"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/quota/image"
)

// NewRegistry returns a registry object that knows how to evaluate quota usage of OpenShift resources.
// Registry for resource quota controller cannot be used with resource quota admission plugin and
// vice versa. If the registry will be used in admission plugin, pass true to forAdmission.
// See a package documentation of pkg/quota/image for more details.
func NewRegistry(osClient osclient.Interface, forAdmission bool) kquota.Registry {
	if forAdmission {
		return image.NewImageRegistryForAdmission(osClient)
	} else {
		return image.NewImageRegistry(osClient)
	}
}
