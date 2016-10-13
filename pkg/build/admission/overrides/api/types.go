package api

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
)

const BuildOverridesPlugin = "BuildOverrides"

// BuildOverridesConfig controls override settings for builds
type BuildOverridesConfig struct {
	unversioned.TypeMeta

	// ForcePull indicates whether the build strategy should always be set to ForcePull=true
	ForcePull bool

	// nodeSelector is a selector which must be true for the build pod to fit on a node
	NodeSelector map[string]string

	// annotations are annotations that will be added to the build pod
	Annotations map[string]string
}

//	TolerationsAnnotationKey string = "scheduler.alpha.kubernetes.io/tolerations"
