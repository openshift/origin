package apps

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/apis/core"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// DeploymentToPodLogOptions builds a PodLogOptions object out of a DeploymentLogOptions.
// Currently DeploymentLogOptions.Container and DeploymentLogOptions.Previous aren't used
// so they won't be copied to PodLogOptions.
//
// Note that Previous for PodLogOptions is different from Previous for DeploymentLogOptions
// so it shouldn't be included here.
func DeploymentToPodLogOptions(opts *DeploymentLogOptions) *kapi.PodLogOptions {
	return &kapi.PodLogOptions{
		Container:    opts.Container,
		Follow:       opts.Follow,
		SinceSeconds: opts.SinceSeconds,
		SinceTime:    opts.SinceTime,
		Timestamps:   opts.Timestamps,
		TailLines:    opts.TailLines,
		LimitBytes:   opts.LimitBytes,
	}
}

// TemplateImage is a structure for helping a caller iterate over a PodSpec
type TemplateImage struct {
	Image string

	Ref *imageapi.DockerImageReference

	From *kapi.ObjectReference

	Container *kapi.Container
}

// templateImageForContainer takes a container and returns a TemplateImage.
func templateImageForContainer(container *kapi.Container, triggerFn TriggeredByFunc) (TemplateImage, error) {
	var ref imageapi.DockerImageReference
	if trigger, ok := triggerFn(container); ok {
		trigger.Image = container.Image
		trigger.Container = container
		return trigger, nil
	}
	ref, err := imageapi.ParseDockerImageReference(container.Image)
	if err != nil {
		return TemplateImage{Image: container.Image, Container: container}, err
	}
	return TemplateImage{Image: container.Image, Ref: &ref, Container: container}, nil
}

// TemplateImageForContainer locates the requested container in a pod spec, returning information about the
// trigger (if it exists), or an error.
func TemplateImageForContainer(pod *kapi.PodSpec, triggerFn TriggeredByFunc, containerName string) (TemplateImage, error) {
	for i := range pod.Containers {
		container := &pod.Containers[i]
		if container.Name != containerName {
			continue
		}
		return templateImageForContainer(container, triggerFn)
	}
	for i := range pod.InitContainers {
		container := &pod.InitContainers[i]
		if container.Name != containerName {
			continue
		}
		return templateImageForContainer(container, triggerFn)
	}
	return TemplateImage{}, fmt.Errorf("no container %q found", containerName)
}

// eachTemplateImage invokes triggerFn and fn on the provided container.
func eachTemplateImage(container *kapi.Container, triggerFn TriggeredByFunc, fn func(TemplateImage, error)) {
	image, err := templateImageForContainer(container, triggerFn)
	fn(image, err)
}

// EachTemplateImage iterates a pod spec, looking for triggers that match each container and invoking
// fn with each located image.
func EachTemplateImage(pod *kapi.PodSpec, triggerFn TriggeredByFunc, fn func(TemplateImage, error)) {
	for i := range pod.Containers {
		eachTemplateImage(&pod.Containers[i], triggerFn, fn)
	}
	for i := range pod.InitContainers {
		eachTemplateImage(&pod.InitContainers[i], triggerFn, fn)
	}
}

// TriggeredByFunc returns a TemplateImage or error from the provided container
type TriggeredByFunc func(container *kapi.Container) (TemplateImage, bool)

// IgnoreTriggers ignores the triggers
func IgnoreTriggers(container *kapi.Container) (TemplateImage, bool) {
	return TemplateImage{}, false
}

// DeploymentConfigHasTrigger returns a function that can identify the image for each container.
func DeploymentConfigHasTrigger(config *DeploymentConfig) TriggeredByFunc {
	return func(container *kapi.Container) (TemplateImage, bool) {
		for _, trigger := range config.Spec.Triggers {
			params := trigger.ImageChangeParams
			if params == nil {
				continue
			}
			for _, name := range params.ContainerNames {
				if container.Name == name {
					if len(params.From.Name) == 0 {
						continue
					}
					from := params.From
					if len(from.Namespace) == 0 {
						from.Namespace = config.Namespace
					}
					return TemplateImage{
						Image: container.Image,
						From:  &from,
					}, true
				}
			}
		}
		return TemplateImage{}, false
	}
}
