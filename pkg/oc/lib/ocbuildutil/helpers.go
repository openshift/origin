package ocbuildutil

import (
	buildv1 "github.com/openshift/api/build/v1"
	corev1 "k8s.io/api/core/v1"
)

// GetInputReference returns the From ObjectReference associated with the
// BuildStrategy.
func GetInputReference(strategy buildv1.BuildStrategy) *corev1.ObjectReference {
	switch {
	case strategy.SourceStrategy != nil:
		return &strategy.SourceStrategy.From
	case strategy.DockerStrategy != nil:
		return strategy.DockerStrategy.From
	case strategy.CustomStrategy != nil:
		return &strategy.CustomStrategy.From
	default:
		return nil
	}
}
