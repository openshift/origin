package common

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api/unversioned"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/controller/policy"
	buildutil "github.com/openshift/origin/pkg/build/util"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
)

func SetBuildCompletionTimeAndDuration(build *buildapi.Build) {
	now := unversioned.Now()
	build.Status.CompletionTimestamp = &now
	if build.Status.StartTimestamp != nil {
		build.Status.Duration = build.Status.CompletionTimestamp.Rfc3339Copy().Time.Sub(build.Status.StartTimestamp.Rfc3339Copy().Time)
	}
}

func HandleBuildCompletion(build *buildapi.Build, runPolicies []policy.RunPolicy) {
	if !buildutil.IsBuildComplete(build) {
		return
	}
	runPolicy := policy.ForBuild(build, runPolicies)
	if runPolicy == nil {
		utilruntime.HandleError(fmt.Errorf("unable to determine build scheduler for %s/%s", build.Namespace, build.Name))
		return
	}
	if err := runPolicy.OnComplete(build); err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to run policy on completed build %s/%s: %v", build.Namespace, build.Name, err))
	}
}

func SetBuildPodNameAnnotation(build *buildapi.Build, podName string) {
	if build.Annotations == nil {
		build.Annotations = map[string]string{}
	}
	build.Annotations[buildapi.BuildPodNameAnnotation] = podName
}

func HasBuildPodNameAnnotation(build *buildapi.Build) bool {
	if build.Annotations == nil {
		return false
	}
	_, hasAnnotation := build.Annotations[buildapi.BuildPodNameAnnotation]
	return hasAnnotation
}
