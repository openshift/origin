package common

import (
	"errors"
	"fmt"

	"github.com/openshift/origin/pkg/build/buildscheme"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	buildv1 "github.com/openshift/api/build/v1"
)

// GetBuildFromPod returns a build object encoded in a pod's BUILD environment variable along with
// its encoding version
func GetBuildFromPod(pod *corev1.Pod) (*buildv1.Build, error) {
	if len(pod.Spec.Containers) == 0 {
		return nil, errors.New("unable to get build from pod: pod has no containers")
	}

	buildEnvVar := getEnvVar(&pod.Spec.Containers[0], "BUILD")
	if len(buildEnvVar) == 0 {
		return nil, errors.New("unable to get build from pod: BUILD environment variable is empty")
	}

	obj, err := runtime.Decode(buildscheme.Decoder, []byte(buildEnvVar))
	if err != nil {
		return nil, fmt.Errorf("unable to get build from pod: %v", err)
	}
	build, ok := obj.(*buildv1.Build)
	if !ok {
		return nil, fmt.Errorf("unable to get build from pod: %v", errors.New("decoded object is not of type Build"))
	}
	return build, nil
}

// SetBuildInPod encodes a build object and sets it in a pod's BUILD environment variable
func SetBuildInPod(pod *corev1.Pod, build *buildv1.Build) error {
	encodedBuild, err := runtime.Encode(buildscheme.Encoder, build)
	if err != nil {
		return fmt.Errorf("unable to set build in pod: %v", err)
	}

	for i := range pod.Spec.Containers {
		setEnvVar(&pod.Spec.Containers[i], "BUILD", string(encodedBuild))
	}
	for i := range pod.Spec.InitContainers {
		setEnvVar(&pod.Spec.InitContainers[i], "BUILD", string(encodedBuild))
	}

	return nil
}

func getEnvVar(c *corev1.Container, name string) string {
	for _, envVar := range c.Env {
		if envVar.Name == name {
			return envVar.Value
		}
	}

	return ""
}

func setEnvVar(c *corev1.Container, name, value string) {
	for i, envVar := range c.Env {
		if envVar.Name == name {
			c.Env[i] = corev1.EnvVar{Name: name, Value: value}
			return
		}
	}

	c.Env = append(c.Env, corev1.EnvVar{Name: name, Value: value})
}
