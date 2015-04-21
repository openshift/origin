package v1beta3

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

// List contains a list of Kubernetes resources to be applied.
// DEPRECATED: Will be replaced with the direct Kubernetes list.
type List struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	// Required: Items is an array of Kubernetes resources of Service,
	// Pod and/or ReplicationController kind.
	Items []runtime.RawExtension `json:"items"`
}
