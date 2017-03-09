package common

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/controller/policy"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

// SetBuildCompletionTimeAndDuration will set the build completion timestamp
// to the current time if it is nil.  It will also set the start timestamp to
// the same value if it is nil.  Returns true if the build object was
// modified.
func SetBuildCompletionTimeAndDuration(build *buildapi.Build) bool {
	if build.Status.CompletionTimestamp != nil {
		return false
	}
	now := metav1.Now()
	build.Status.CompletionTimestamp = &now
	// apparently this build completed so fast we didn't see the pod running event,
	// so just use the completion time as the start time.
	if build.Status.StartTimestamp == nil {
		build.Status.StartTimestamp = &now
	}
	build.Status.Duration = build.Status.CompletionTimestamp.Rfc3339Copy().Time.Sub(build.Status.StartTimestamp.Rfc3339Copy().Time)
	return true
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
