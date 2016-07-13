package admission

import (
	"errors"
	"fmt"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// IsBuildPod returns true if a pod is a pod generated for a Build
func IsBuildPod(a admission.Attributes) bool {
	if a.GetResource().GroupResource() != kapi.Resource("pods") {
		return false
	}
	if len(a.GetSubresource()) != 0 {
		return false
	}
	pod, err := GetPod(a)
	if err != nil {
		return false
	}
	return hasBuildAnnotation(pod) && hasBuildEnvVar(pod)
}

// GetBuild returns a build object encoded in a pod's BUILD environment variable along with
// its encoding version
func GetBuild(a admission.Attributes) (*buildapi.Build, unversioned.GroupVersion, error) {
	pod, err := GetPod(a)
	if err != nil {
		return nil, unversioned.GroupVersion{}, err
	}
	build, version, err := getBuildFromPod(pod)
	if err != nil {
		return nil, unversioned.GroupVersion{}, admission.NewForbidden(a, fmt.Errorf("unable to get build from pod: %v", err))
	}
	return build, version, nil
}

// GetPod returns a pod from an admission attributes object
func GetPod(a admission.Attributes) (*kapi.Pod, error) {
	pod, isPod := a.GetObject().(*kapi.Pod)
	if !isPod {
		return nil, admission.NewForbidden(a, fmt.Errorf("unrecognized request object: %#v", a.GetObject()))
	}
	return pod, nil
}

// SetBuild encodes a build object and sets it in a pod's BUILD environment variable
func SetBuild(a admission.Attributes, build *buildapi.Build, groupVersion unversioned.GroupVersion) error {
	pod, err := GetPod(a)
	if err != nil {
		return err
	}

	err = setBuildInPod(build, pod, groupVersion)
	if err != nil {
		return admission.NewForbidden(a, fmt.Errorf("unable to set build in pod: %v", err))
	}

	return nil
}

// SetBuildLogLevel extracts BUILD_LOGLEVEL from the Build environment
// and feeds it as an argument to the Pod's entrypoint. The BUILD_LOGLEVEL
// environment variable may have been set in multiple ways: a default value,
// by a BuildConfig, or by the BuildDefaults admission plugin. In this method
// we finally act on the value by injecting it into the Pod.
func SetBuildLogLevel(attributes admission.Attributes, build *buildapi.Build) error {
	pod, err := GetPod(attributes)
	if err != nil {
		return err
	}

	var envs []kapi.EnvVar

	// Check whether the build strategy supports --loglevel parameter.
	switch {
	case build.Spec.Strategy.DockerStrategy != nil:
		envs = build.Spec.Strategy.DockerStrategy.Env
	case build.Spec.Strategy.SourceStrategy != nil:
		envs = build.Spec.Strategy.SourceStrategy.Env
	default:
		// The build strategy does not support --loglevel
		return nil
	}

	buildLogLevel := "0" // The ultimate default for the build pod's loglevel if no actor sets BUILD_LOGLEVEL in the Build
	for i := range envs {
		env := envs[i]
		if env.Name == "BUILD_LOGLEVEL" {
			buildLogLevel = env.Value
			break
		}
	}
	c := &pod.Spec.Containers[0]
	c.Args = append(c.Args, "--loglevel="+buildLogLevel)
	return nil
}

// getBuildFromPod detects the encoding of a Build in a pod and returns the Build
// object along with its detected version.
func getBuildFromPod(pod *kapi.Pod) (*buildapi.Build, unversioned.GroupVersion, error) {
	envVar, err := buildEnvVar(pod)
	if err != nil {
		return nil, unversioned.GroupVersion{}, err
	}
	obj, groupVersionKind, err := kapi.Codecs.UniversalDecoder().Decode([]byte(envVar.Value), nil, nil)
	if err != nil {
		return nil, unversioned.GroupVersion{}, err
	}
	build, ok := obj.(*buildapi.Build)
	if !ok {
		return nil, unversioned.GroupVersion{}, errors.New("decoded object is not of type Build")
	}
	return build, groupVersionKind.GroupVersion(), nil
}

// setBuildInPod encodes a build with the given version and sets it in the BUILD environment variable
// of the pod.
func setBuildInPod(build *buildapi.Build, pod *kapi.Pod, groupVersion unversioned.GroupVersion) error {
	envVar, err := buildEnvVar(pod)
	if err != nil {
		return err
	}
	encodedBuild, err := runtime.Encode(kapi.Codecs.LegacyCodec(groupVersion), build)
	if err != nil {
		return err
	}
	envVar.Value = string(encodedBuild)
	return nil
}

func buildEnvVar(pod *kapi.Pod) (*kapi.EnvVar, error) {
	if len(pod.Spec.Containers) == 0 {
		return nil, errors.New("pod has no containers")
	}
	env := pod.Spec.Containers[0].Env
	for i := range env {
		if env[i].Name == "BUILD" {
			if len(env[i].Value) == 0 {
				return nil, errors.New("BUILD environment variable is empty")
			}
			return &env[i], nil
		}
	}
	return nil, errors.New("pod does not have a BUILD environment variable")
}

func hasBuildEnvVar(pod *kapi.Pod) bool {
	_, err := buildEnvVar(pod)
	return err == nil
}

func hasBuildAnnotation(pod *kapi.Pod) bool {
	if pod.Annotations == nil {
		return false
	}
	_, hasAnnotation := pod.Annotations[buildapi.BuildAnnotation]
	return hasAnnotation
}
