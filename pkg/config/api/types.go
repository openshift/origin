package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

// Config contains a set of Kubernetes resources to be applied.
// TODO: Unify with Kubernetes Config
//       https://github.com/GoogleCloudPlatform/kubernetes/pull/1007
type Config struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:",inline" yaml:",inline"`

	// Required: Items is an array of Kubernetes resources of Service,
	// Pod and/or ReplicationController kind.
	// TODO: Handle unregistered types. Define custom []runtime.Object
	//       type and its unmarshaller instead of []runtime.Object.
	Items []runtime.RawExtension `json:"items" yaml:"items"`
}
