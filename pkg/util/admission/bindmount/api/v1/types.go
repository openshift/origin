package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// BinaryBindMountConfig is the configuration for the BinaryBindMount
// admission controller which overrides binaries in certain images executed by the system
type BinaryBindMountConfig struct {
	unversioned.TypeMeta `json:",inline"`

	Images []ImageBindMountSpec `json:"images"`
}

type ImageBindMountSpec struct {
	Image  string              `json:"image"`
	Mounts []FileBindMountSpec `json:"mounts"`
}

type FileBindMountSpec struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}
