package common

import (
	"fmt"
	"sort"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/third_party/forked/golang/expansion"

	buildv1 "github.com/openshift/api/build/v1"
	buildlister "github.com/openshift/client-go/build/listers/build/v1"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/pod/envresolve"
)

type ByCreationTimestamp []*buildv1.Build

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
	glog.V(4).Infof("Handling build pruning for %s/%s", namespace, buildConfigName)

	buildConfig, err := buildConfigGetter.BuildConfigs(namespace).Get(buildConfigName)
	if err != nil {
		return err
	}

	var buildsToDelete []*buildv1.Build
	var errList []error

	if buildConfig.Spec.SuccessfulBuildsHistoryLimit != nil {
		successfulBuilds, err := buildutil.BuildConfigBuilds(buildLister, namespace, buildConfigName, func(build *buildv1.Build) bool { return build.Status.Phase == buildv1.BuildPhaseComplete })
		if err != nil {
			return err
		}
		sort.Sort(ByCreationTimestamp(successfulBuilds))

		successfulBuildsHistoryLimit := int(*buildConfig.Spec.SuccessfulBuildsHistoryLimit)
		glog.V(5).Infof("Current successful builds: %v, SuccessfulBuildsHistoryLimit: %v", len(successfulBuilds), successfulBuildsHistoryLimit)
		if len(successfulBuilds) > successfulBuildsHistoryLimit {
			glog.V(5).Infof("Preparing to prune %v of %v successful builds", len(successfulBuilds)-successfulBuildsHistoryLimit, len(successfulBuilds))
			buildsToDelete = append(buildsToDelete, successfulBuilds[successfulBuildsHistoryLimit:]...)
		}
	}

	if buildConfig.Spec.FailedBuildsHistoryLimit != nil {
		failedBuilds, err := buildutil.BuildConfigBuilds(buildLister, namespace, buildConfigName, func(build *buildv1.Build) bool {
			return build.Status.Phase == buildv1.BuildPhaseFailed || build.Status.Phase == buildv1.BuildPhaseCancelled || build.Status.Phase == buildv1.BuildPhaseError
		})
		if err != nil {
			return err
		}
		sort.Sort(ByCreationTimestamp(failedBuilds))

		failedBuildsHistoryLimit := int(*buildConfig.Spec.FailedBuildsHistoryLimit)
		glog.V(5).Infof("Current failed builds: %v, FailedBuildsHistoryLimit: %v", len(failedBuilds), failedBuildsHistoryLimit)
		if len(failedBuilds) > failedBuildsHistoryLimit {
			glog.V(5).Infof("Preparing to prune %v of %v failed builds", len(failedBuilds)-failedBuildsHistoryLimit, len(failedBuilds))
			buildsToDelete = append(buildsToDelete, failedBuilds[failedBuildsHistoryLimit:]...)
		}
	}

	for i, b := range buildsToDelete {
		glog.V(4).Infof("Pruning build: %s/%s", b.Namespace, b.Name)
		if err := buildDeleter.DeleteBuild(buildsToDelete[i]); err != nil {
			errList = append(errList, err)
		}
	}
	if errList != nil {
		return kerrors.NewAggregate(errList)
	}

	return nil
}

func SetBuildPodNameAnnotation(build *buildv1.Build, podName string) {
	if build.Annotations == nil {
		build.Annotations = map[string]string{}
	}
	build.Annotations[buildutil.BuildPodNameAnnotation] = podName
}

func HasBuildPodNameAnnotation(build *buildv1.Build) bool {
	if build.Annotations == nil {
		return false
	}
	_, hasAnnotation := build.Annotations[buildutil.BuildPodNameAnnotation]
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
func ResolveValueFrom(pod *corev1.Pod, client kubernetes.Interface) error {
	var outputEnv []corev1.EnvVar
	var allErrs []error

	build, err := GetBuildFromPod(pod)
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

		outputEnv = append(outputEnv, corev1.EnvVar{Name: e.Name, Value: value})
		mapEnvs[e.Name] = value
	}

	if len(allErrs) > 0 {
		return ErrEnvVarResolver{utilerrors.NewAggregate(allErrs)}
	}

	buildutil.SetBuildEnv(build, outputEnv)
	return SetBuildInPod(pod, build)
}
