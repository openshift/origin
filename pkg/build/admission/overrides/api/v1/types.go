package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// BuildOverridesConfig controls override settings for builds
type BuildOverridesConfig struct {
	unversioned.TypeMeta `json:",inline"`

	// ForcePull indicates whether the build strategy should always be set to ForcePull=true
	ForcePull bool `json:"forcePull",description:"if true, will always set ForcePull to true on builds"`
}
