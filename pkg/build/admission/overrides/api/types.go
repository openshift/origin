package api

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// BuildOverridesConfig controls override settings for builds
type BuildOverridesConfig struct {
	unversioned.TypeMeta

	// ForcePull indicates whether the build strategy should always be set to ForcePull=true
	ForcePull bool
}
