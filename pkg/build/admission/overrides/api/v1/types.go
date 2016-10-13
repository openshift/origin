package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// BuildOverridesConfig controls override settings for builds
type BuildOverridesConfig struct {
	unversioned.TypeMeta `json:",inline"`

	// ForcePull indicates whether the build strategy should always be set to ForcePull=true
	ForcePull bool `json:"forcePull"`

	// nodeSelector is a selector which must be true for the build pod to fit on a node
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// annotations are annotations that will be added to the build pod
	Annotations map[string]string `json:"annotations,omitempty"`
}
