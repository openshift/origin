package common

import (
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/third_party/forked/golang/expansion"

	buildadmission "github.com/openshift/origin/pkg/build/admission"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/client"
	"github.com/openshift/origin/pkg/build/controller/policy"
	buildlister "github.com/openshift/origin/pkg/build/generated/listers/build/internalversion"
	buildutil "github.com/openshift/origin/pkg/build/util"
	envresolve "github.com/openshift/origin/pkg/pod/envresolve"

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

func HandleBuildCompletion(build *buildapi.Build, buildLister buildlister.BuildLister, buildConfigGetter buildlister.BuildConfigLister, buildDeleter buildclient.BuildDeleter, runPolicies []policy.RunPolicy) {
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

type ByCreationTimestamp []*buildapi.Build

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
func HandleBuildPruning(buildConfigName string, namespace string, buildLister buildlister.BuildLister, buildConfigGetter buildlister.BuildConfigLister, buildDeleter buildclient.BuildDeleter) error {
	buildConfig, err := buildConfigGetter.BuildConfigs(namespace).Get(buildConfigName)
	if err != nil {
		return err
	}

	var buildsToDelete []*buildapi.Build
	var errList []error

	if buildConfig.Spec.SuccessfulBuildsHistoryLimit != nil {
		successfulBuilds, err := buildutil.BuildConfigBuilds(buildLister, namespace, buildConfigName, func(build *buildapi.Build) bool { return build.Status.Phase == buildapi.BuildPhaseComplete })
		if err != nil {
			return err
		}
		sort.Sort(ByCreationTimestamp(successfulBuilds))

		successfulBuildsHistoryLimit := int(*buildConfig.Spec.SuccessfulBuildsHistoryLimit)
		if len(successfulBuilds) > successfulBuildsHistoryLimit {
			glog.V(4).Infof("Preparing to prune %v of %v old successful builds, successfulBuildsHistoryLimit set to %v", (len(successfulBuilds) - successfulBuildsHistoryLimit), len(successfulBuilds), successfulBuildsHistoryLimit)
			buildsToDelete = append(buildsToDelete, successfulBuilds[successfulBuildsHistoryLimit:]...)
		}
	}

	if buildConfig.Spec.FailedBuildsHistoryLimit != nil {
		failedBuilds, err := buildutil.BuildConfigBuilds(buildLister, namespace, buildConfigName, func(build *buildapi.Build) bool {
			return (build.Status.Phase == buildapi.BuildPhaseFailed || build.Status.Phase == buildapi.BuildPhaseCancelled || build.Status.Phase == buildapi.BuildPhaseError)
		})
		if err != nil {
			return err
		}
		sort.Sort(ByCreationTimestamp(failedBuilds))

		failedBuildsHistoryLimit := int(*buildConfig.Spec.FailedBuildsHistoryLimit)
		if len(failedBuilds) > failedBuildsHistoryLimit {
			glog.V(4).Infof("Preparing to prune %v of %v old failed builds, failedBuildsHistoryLimit set to %v", (len(failedBuilds) - failedBuildsHistoryLimit), len(failedBuilds), failedBuildsHistoryLimit)
			buildsToDelete = append(buildsToDelete, failedBuilds[failedBuildsHistoryLimit:]...)
		}
	}

	for i, b := range buildsToDelete {
		glog.V(4).Infof("Pruning old build: %s/%s", b.Namespace, b.Name)
		if err := buildDeleter.DeleteBuild(buildsToDelete[i]); err != nil {
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

// ErrEnvVarResolver is an error type for build environment resolution errors
type ErrEnvVarResolver struct {
	message kerrors.Aggregate
}

// Error returns a string representation of the error
func (e ErrEnvVarResolver) Error() string {
	return fmt.Sprintf("%v", e.message)
}

// ResolveValueFrom resolves valueFrom references in build environment variables
// including references to existing environment variables, jsonpath references to
// the build object, secrets, and configmaps.
// The build.Strategy.BuildStrategy.Env is replaced with the resolved references.
func ResolveValueFrom(pod *v1.Pod, client kclientset.Interface) error {
	var outputEnv []kapi.EnvVar
	var allErrs []error

	build, version, err := buildadmission.GetBuildFromPod(pod)
	if err != nil {
		return nil
	}

	mapEnvs := map[string]string{}
	mapping := expansion.MappingFuncFor(mapEnvs)
	inputEnv := buildutil.GetBuildEnv(build)
	store := envresolve.NewResourceStore()

	for _, e := range inputEnv {
		var value string
		var err error

		if e.Value != "" {
			value = expansion.Expand(e.Value, mapping)
		} else if e.ValueFrom != nil {
			value, err = envresolve.GetEnvVarRefValue(client, build.Namespace, store, e.ValueFrom, build, nil)
			if err != nil {
				allErrs = append(allErrs, err)
				continue
			}
		}

		outputEnv = append(outputEnv, kapi.EnvVar{Name: e.Name, Value: value})
		mapEnvs[e.Name] = value
	}

	if len(allErrs) > 0 {
		return ErrEnvVarResolver{utilerrors.NewAggregate(allErrs)}
	}

	buildutil.SetBuildEnv(build, outputEnv)
	return buildadmission.SetBuildInPod(pod, build, version)
}
