package internal_helpers

import (
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/origin/pkg/api/apihelpers"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

// NOTE: These helpers are used by apiserver only as the apiserver use the internal types.
//       These were copied from pkg/build/util and any change to original helpers should be reflected here as well.

const (
	// buildPodSuffix is the suffix used to append to a build pod name given a build name
	buildPodSuffix = "build"
)

// BuildToPodLogOptions builds a PodLogOptions object out of a BuildLogOptions.
// Currently BuildLogOptions.Container and BuildLogOptions.Previous aren't used
// so they won't be copied to PodLogOptions.
// DEPRECATED: Reserved for apiserver, do not use outside of it
func BuildToPodLogOptions(opts *buildapi.BuildLogOptions) *kapi.PodLogOptions {
	return &kapi.PodLogOptions{
		Follow:       opts.Follow,
		SinceSeconds: opts.SinceSeconds,
		SinceTime:    opts.SinceTime,
		Timestamps:   opts.Timestamps,
		TailLines:    opts.TailLines,
		LimitBytes:   opts.LimitBytes,
	}
}

// DEPRECATED: Reserved for apiserver, do not use outside of it
func IsBuildComplete(b *buildapi.Build) bool {
	return IsTerminalPhase(b.Status.Phase)
}

// DEPRECATED: Reserved for apiserver, do not use outside of it
func IsTerminalPhase(p buildapi.BuildPhase) bool {
	switch p {
	case buildapi.BuildPhaseNew,
		buildapi.BuildPhasePending,
		buildapi.BuildPhaseRunning:
		return false
	}
	return true
}

// GetBuildPodName returns name of the build pod.
// DEPRECATED: Reserved for apiserver, do not use outside of it
func GetBuildPodName(build *buildapi.Build) string {
	return apihelpers.GetPodName(build.Name, buildPodSuffix)
}

// DEPRECATED: Reserved for apiserver, do not use outside of it
func StrategyType(strategy buildapi.BuildStrategy) string {
	switch {
	case strategy.DockerStrategy != nil:
		return "Docker"
	case strategy.CustomStrategy != nil:
		return "Custom"
	case strategy.SourceStrategy != nil:
		return "Source"
	case strategy.JenkinsPipelineStrategy != nil:
		return "JenkinsPipeline"
	}
	return ""
}

// GetInputReference returns the From ObjectReference associated with the
// BuildStrategy.
// DEPRECATED: Reserved for apiserver, do not use outside of it
func GetInputReference(strategy buildapi.BuildStrategy) *kapi.ObjectReference {
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

// GetBuildEnv gets the build strategy environment
// DEPRECATED: Reserved for apiserver, do not use outside of it
func GetBuildEnv(build *buildapi.Build) []kapi.EnvVar {
	switch {
	case build.Spec.Strategy.SourceStrategy != nil:
		return build.Spec.Strategy.SourceStrategy.Env
	case build.Spec.Strategy.DockerStrategy != nil:
		return build.Spec.Strategy.DockerStrategy.Env
	case build.Spec.Strategy.CustomStrategy != nil:
		return build.Spec.Strategy.CustomStrategy.Env
	case build.Spec.Strategy.JenkinsPipelineStrategy != nil:
		return build.Spec.Strategy.JenkinsPipelineStrategy.Env
	default:
		return nil
	}
}

// SetBuildEnv replaces the current build environment
// DEPRECATED: Reserved for apiserver, do not use outside of it
func SetBuildEnv(build *buildapi.Build, env []kapi.EnvVar) {
	var oldEnv *[]kapi.EnvVar

	switch {
	case build.Spec.Strategy.SourceStrategy != nil:
		oldEnv = &build.Spec.Strategy.SourceStrategy.Env
	case build.Spec.Strategy.DockerStrategy != nil:
		oldEnv = &build.Spec.Strategy.DockerStrategy.Env
	case build.Spec.Strategy.CustomStrategy != nil:
		oldEnv = &build.Spec.Strategy.CustomStrategy.Env
	case build.Spec.Strategy.JenkinsPipelineStrategy != nil:
		oldEnv = &build.Spec.Strategy.JenkinsPipelineStrategy.Env
	default:
		return
	}
	*oldEnv = env
}

// UpdateBuildEnv updates the strategy environment
// This will replace the existing variable definitions with provided env
// DEPRECATED: Reserved for apiserver, do not use outside of it
func UpdateBuildEnv(build *buildapi.Build, env []kapi.EnvVar) {
	buildEnv := GetBuildEnv(build)

	newEnv := []kapi.EnvVar{}
	for _, e := range buildEnv {
		exists := false
		for _, n := range env {
			if e.Name == n.Name {
				exists = true
				break
			}
		}
		if !exists {
			newEnv = append(newEnv, e)
		}
	}
	newEnv = append(newEnv, env...)
	SetBuildEnv(build, newEnv)
}

// BuildSliceByCreationTimestamp implements sort.Interface for []Build
// based on the CreationTimestamp field.
// DEPRECATED: Reserved for apiserver, do not use outside of it
// +k8s:deepcopy-gen=false
type BuildSliceByCreationTimestamp []buildapi.Build

func (b BuildSliceByCreationTimestamp) Len() int {
	return len(b)
}

func (b BuildSliceByCreationTimestamp) Less(i, j int) bool {
	return b[i].CreationTimestamp.Before(&b[j].CreationTimestamp)
}

func (b BuildSliceByCreationTimestamp) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}
