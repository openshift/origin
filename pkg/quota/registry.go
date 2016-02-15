package quota

import (
	kquota "k8s.io/kubernetes/pkg/quota"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/quota/image"
)

// NewRegistry returns a registry object that knows how to evaluate quota usage of OpenShift resources.
func NewRegistry(osClient osclient.Interface) kquota.Registry {
	return image.NewImageRegistry(osClient)
}
