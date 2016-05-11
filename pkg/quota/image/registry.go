// Package image implements evaluators of usage for imagestreams and images. They are supposed
// to be passed to resource quota controller and origin resource quota admission plugin.
package image

import (
	"k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/generic"

	osclient "github.com/openshift/origin/pkg/client"
)

// NewImageRegistry returns a registry for quota evaluation of OpenShift resources related to images in
// internal registry.
func NewImageRegistry(osClient osclient.Interface) quota.Registry {
	return &generic.GenericRegistry{}
}
