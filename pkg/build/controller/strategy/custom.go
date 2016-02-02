package strategy

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

// CustomBuildStrategy creates a build using a custom builder image.
type CustomBuildStrategy struct {
	// Codec is the codec to use for encoding the output pod.
	// IMPORTANT: This may break backwards compatibility when
	// it changes.
	Codec runtime.Codec
}

// CreateBuildPod creates the pod to be used for the Custom build
func (bs *CustomBuildStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	strategy := build.Spec.Strategy.CustomStrategy

	codec := bs.Codec
	if len(strategy.BuildAPIVersion) != 0 {
		gv, err := unversioned.ParseGroupVersion(strategy.BuildAPIVersion)
		if err != nil {
			return nil, FatalError(fmt.Sprintf("failed to parse buildAPIVersion specified in custom build strategy (%q): %v", strategy.BuildAPIVersion, err))
		}
		interfaces, err := latest.InterfacesFor(gv)
		if err != nil {
			return nil, FatalError(fmt.Sprintf("invalid buildAPIVersion specified in custom build strategy (%q): %v", strategy.BuildAPIVersion, err))
		}
		codec = interfaces.Codec
	}

	data, err := codec.Encode(build)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the build: %v", err)
	}

	containerEnv := []kapi.EnvVar{{Name: "BUILD", Value: string(data)}}

	if build.Spec.Source.Git != nil {
		addSourceEnvVars(build.Spec.Source, &containerEnv)
	}
	addOriginVersionVar(&containerEnv)

	if strategy == nil || len(strategy.From.Name) == 0 {
		return nil, errors.New("CustomBuildStrategy cannot be executed without image")
	}

	if len(strategy.Env) > 0 {
		containerEnv = append(containerEnv, strategy.Env...)
	}

	if strategy.ExposeDockerSocket {
		glog.V(2).Infof("ExposeDockerSocket is enabled for %s build", build.Name)
		containerEnv = append(containerEnv, kapi.EnvVar{Name: "DOCKER_SOCKET", Value: dockerSocketPath})
	}

	privileged := true
	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name:      buildutil.GetBuildPodName(build),
			Namespace: build.Namespace,
			Labels:    getPodLabels(build),
		},
		Spec: kapi.PodSpec{
			ServiceAccountName: build.Spec.ServiceAccount,
			Containers: []kapi.Container{
				{
					Name:  "custom-build",
					Image: strategy.From.Name,
					Env:   containerEnv,
					// TODO: run unprivileged https://github.com/openshift/origin/issues/662
					SecurityContext: &kapi.SecurityContext{
						Privileged: &privileged,
					},
				},
			},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}
	if build.Spec.CompletionDeadlineSeconds != nil {
		pod.Spec.ActiveDeadlineSeconds = build.Spec.CompletionDeadlineSeconds
	}

	if err := setupBuildEnv(build, pod); err != nil {
		return nil, err
	}

	if !strategy.ForcePull {
		pod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent
	} else {
		glog.V(2).Infof("ForcePull is enabled for %s build", build.Name)
		pod.Spec.Containers[0].ImagePullPolicy = kapi.PullAlways
	}
	pod.Spec.Containers[0].Resources = build.Spec.Resources
	if build.Spec.Source.Binary != nil {
		pod.Spec.Containers[0].Stdin = true
		pod.Spec.Containers[0].StdinOnce = true
	}

	if strategy.ExposeDockerSocket {
		setupDockerSocket(pod)
		setupDockerSecrets(pod, build.Spec.Output.PushSecret, strategy.PullSecret, build.Spec.Source.Images)
	}
	setupSourceSecrets(pod, build.Spec.Source.SourceSecret)
	setupSecrets(pod, build.Spec.Source.Secrets)
	setupAdditionalSecrets(pod, build.Spec.Strategy.CustomStrategy.Secrets)
	return pod, nil
}
