package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"

	buildapi "github.com/openshift/origin/pkg/build/api/v1"
)

// BuildOverridesConfig controls override settings for builds
type BuildOverridesConfig struct {
	unversioned.TypeMeta `json:",inline"`

	// ForcePull indicates whether the build strategy should always be set to ForcePull=true
	ForcePull bool `json:"forcePull"`

	// ImageLabels is a list of docker labels that are applied to the resulting image.
	// If user provided a label in their Build/BuildConfig with the same name as one in this
	// list, the user's label will be overwritten.
	ImageLabels []buildapi.ImageLabel `json:"imageLabels,omitempty"`
}
