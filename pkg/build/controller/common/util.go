package common

import (
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	"github.com/openshift/origin/pkg/build/controller/policy"
	buildutil "github.com/openshift/origin/pkg/build/util"

	"github.com/golang/glog"
)

// SetBuildCompletionTimeAndDuration will set the build completion timestamp
// to the current time if it is nil.  It will also set the start timestamp to
// the same value if it is nil.  Returns true if the build object was
// modified.
func SetBuildCompletionTimeAndDuration(build *buildapi.Build, startTime *metav1.Time) bool {
	if build.Status.CompletionTimestamp != nil {
		return false
	}
	now := metav1.Now()
	build.Status.CompletionTimestamp = &now
	// apparently this build completed so fast we didn't see the pod running event,
	// so just use the pod start time as the start time.
	if build.Status.StartTimestamp == nil {
		build.Status.StartTimestamp = startTime
	}
	if build.Status.StartTimestamp != nil {
		build.Status.Duration = build.Status.CompletionTimestamp.Rfc3339Copy().Time.Sub(build.Status.StartTimestamp.Rfc3339Copy().Time)
	}
	return true
}

func HandleBuildCompletion(build *buildapi.Build, buildLister buildclient.BuildLister, buildConfigGetter buildclient.BuildConfigGetter, buildDeleter buildclient.BuildDeleter, runPolicies []policy.RunPolicy) {
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
	if err := HandleBuildPruning(buildutil.ConfigNameForBuild(build), build.Namespace, buildLister, buildConfigGetter, buildDeleter); err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to prune old builds %s/%s: %v", build.Namespace, build.Name, err))
	}
}

type ByCreationTimestamp []buildapi.Build

func (b ByCreationTimestamp) Len() int {
	return len(b)
}

func (b ByCreationTimestamp) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b ByCreationTimestamp) Less(i, j int) bool {
	return !b[j].CreationTimestamp.Time.After(b[i].CreationTimestamp.Time)
}

// HandleBuildPruning handles the deletion of old successful and failed builds
// based on settings in the BuildConfig.
func HandleBuildPruning(buildConfigName string, namespace string, buildLister buildclient.BuildLister, buildConfigGetter buildclient.BuildConfigGetter, buildDeleter buildclient.BuildDeleter) error {
	buildConfig, err := buildConfigGetter.Get(namespace, buildConfigName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var buildsToDelete []buildapi.Build
	var errList []error

	if buildConfig.Spec.SuccessfulBuildsHistoryLimit != nil {
		successfulBuilds, err := buildutil.BuildConfigBuilds(buildLister, namespace, buildConfigName, func(build buildapi.Build) bool { return build.Status.Phase == buildapi.BuildPhaseComplete })
		if err != nil {
			return err
		}
		sort.Sort(ByCreationTimestamp(successfulBuilds.Items))

		successfulBuildsHistoryLimit := int(*buildConfig.Spec.SuccessfulBuildsHistoryLimit)
		if len(successfulBuilds.Items) > successfulBuildsHistoryLimit {
			glog.V(4).Infof("Preparing to prune %v of %v old successful builds, successfulBuildsHistoryLimit set to %v", (len(successfulBuilds.Items) - successfulBuildsHistoryLimit), len(successfulBuilds.Items), successfulBuildsHistoryLimit)
			buildsToDelete = append(buildsToDelete, successfulBuilds.Items[successfulBuildsHistoryLimit:]...)
		}
	}

	if buildConfig.Spec.FailedBuildsHistoryLimit != nil {
		failedBuilds, err := buildutil.BuildConfigBuilds(buildLister, namespace, buildConfigName, func(build buildapi.Build) bool {
			return (build.Status.Phase == buildapi.BuildPhaseFailed || build.Status.Phase == buildapi.BuildPhaseCancelled || build.Status.Phase == buildapi.BuildPhaseError)
		})
		if err != nil {
			return err
		}
		sort.Sort(ByCreationTimestamp(failedBuilds.Items))

		failedBuildsHistoryLimit := int(*buildConfig.Spec.FailedBuildsHistoryLimit)
		if len(failedBuilds.Items) > failedBuildsHistoryLimit {
			glog.V(4).Infof("Preparing to prune %v of %v old failed builds, failedBuildsHistoryLimit set to %v", (len(failedBuilds.Items) - failedBuildsHistoryLimit), len(failedBuilds.Items), failedBuildsHistoryLimit)
			buildsToDelete = append(buildsToDelete, failedBuilds.Items[failedBuildsHistoryLimit:]...)
		}
	}

	for i, b := range buildsToDelete {
		glog.V(4).Infof("Pruning old build: %s/%s", b.Namespace, b.Name)
		if err := buildDeleter.DeleteBuild(&buildsToDelete[i]); err != nil {
			errList = append(errList, err)
		}
	}
	if errList != nil {
		return kerrors.NewAggregate(errList)
	}

	return nil
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
