package graph

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func belongsToDeploymentConfig(config *deployapi.DeploymentConfig, b *kapi.ReplicationController) bool {
	if b.Annotations != nil {
		return config.Name == deployutil.DeploymentConfigNameFor(b)
	}
	return false
}

type RecentDeploymentReferences []*kapi.ReplicationController

func (m RecentDeploymentReferences) Len() int      { return len(m) }
func (m RecentDeploymentReferences) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m RecentDeploymentReferences) Less(i, j int) bool {
	return deployutil.DeploymentVersionFor(m[i]) > deployutil.DeploymentVersionFor(m[j])
}

// TODO: move to deploy/api/helpers.go
type TemplateImage struct {
	Image string

	Ref *imageapi.DockerImageReference

	From    *kapi.ObjectReference
	FromTag string
}

func EachTemplateImage(pod *kapi.PodSpec, triggerFn TriggeredByFunc, fn func(TemplateImage, error)) {
	for _, container := range pod.Containers {
		var ref imageapi.DockerImageReference
		if trigger, ok := triggerFn(&container); ok {
			trigger.Image = container.Image
			fn(trigger, nil)
			continue
		}
		ref, err := imageapi.ParseDockerImageReference(container.Image)
		if err != nil {
			fn(TemplateImage{Image: container.Image}, err)
			continue
		}
		fn(TemplateImage{Image: container.Image, Ref: &ref}, nil)
	}
}

type TriggeredByFunc func(container *kapi.Container) (TemplateImage, bool)

func DeploymentConfigHasTrigger(config *deployapi.DeploymentConfig) TriggeredByFunc {
	return func(container *kapi.Container) (TemplateImage, bool) {
		for _, trigger := range config.Triggers {
			params := trigger.ImageChangeParams
			if params == nil {
				continue
			}
			for _, name := range params.ContainerNames {
				if container.Name == name {
					if len(params.From.Name) == 0 {
						continue
					}
					tag := params.Tag
					if len(tag) == 0 {
						tag = imageapi.DefaultImageTag
					}
					from := params.From
					if len(from.Namespace) == 0 {
						from.Namespace = config.Namespace
					}
					return TemplateImage{
						Image:   container.Image,
						From:    &from,
						FromTag: tag,
					}, true
				}
			}
		}
		return TemplateImage{}, false
	}
}
