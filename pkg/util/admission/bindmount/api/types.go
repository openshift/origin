package api

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// BinaryBindMountConfig is the configuration for the BinaryBindMount
// admission controller which overrides binaries in certain images executed by the system
type BinaryBindMountConfig struct {
	unversioned.TypeMeta

	Images []ImageBindMountSpec
}

type ImageBindMountSpec struct {
	Image  string
	Mounts []FileBindMountSpec
}

type FileBindMountSpec struct {
	Source      string
	Destination string
}
