package api

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

const BuildOverridesPlugin = "BuildOverrides"

// BuildOverridesConfig controls override settings for builds
type BuildOverridesConfig struct {
	metav1.TypeMeta

	// forcePull indicates whether the build strategy should always be set to ForcePull=true
	ForcePull bool

	// imageLabels is a list of docker labels that are applied to the resulting image.
	// If user provided a label in their Build/BuildConfig with the same name as one in this
	// list, the user's label will be overwritten.
	ImageLabels []buildapi.ImageLabel

	// nodeSelector is a selector which must be true for the build pod to fit on a node
	NodeSelector map[string]string

	// annotations are annotations that will be added to the build pod
	Annotations map[string]string
}
