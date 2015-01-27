package v1beta1

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

// Config contains a list of Kubernetes resources to be applied.
// DEPRECATED: All clients should use Kubernetes api.List instead of Config.
type Config struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Required: Items is an array of Kubernetes resources of Service,
	// Pod and/or ReplicationController kind.
	Items []runtime.RawExtension `json:"items"`
}
