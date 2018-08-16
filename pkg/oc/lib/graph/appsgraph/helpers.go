package appsgraph

import (
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/openshift/api/apps/v1"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	appsgraph "github.com/openshift/origin/pkg/oc/lib/graph/appsgraph/nodes"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
	kubegraph "github.com/openshift/origin/pkg/oc/lib/graph/kubegraph/nodes"
)

// RelevantDeployments returns the active deployment and a list of inactive deployments (in order from newest to oldest)
func RelevantDeployments(g osgraph.Graph, dcNode *appsgraph.DeploymentConfigNode) (*kubegraph.ReplicationControllerNode, []*kubegraph.ReplicationControllerNode) {
	allDeployments := []*kubegraph.ReplicationControllerNode{}
	uncastDeployments := g.SuccessorNodesByEdgeKind(dcNode, DeploymentEdgeKind)
	if len(uncastDeployments) == 0 {
		return nil, []*kubegraph.ReplicationControllerNode{}
	}

	for i := range uncastDeployments {
		allDeployments = append(allDeployments, uncastDeployments[i].(*kubegraph.ReplicationControllerNode))
	}

	sort.Sort(RecentDeploymentReferences(allDeployments))

	if dcNode.DeploymentConfig.Status.LatestVersion == appsutil.DeploymentVersionFor(allDeployments[0].ReplicationController) {
		return allDeployments[0], allDeployments[1:]
	}

	return nil, allDeployments
}

func BelongsToDeploymentConfig(config *appsv1.DeploymentConfig, b *corev1.ReplicationController) bool {
	if b.Annotations != nil {
		return config.Name == appsutil.DeploymentConfigNameFor(b)
	}
	return false
}

type RecentDeploymentReferences []*kubegraph.ReplicationControllerNode

func (m RecentDeploymentReferences) Len() int      { return len(m) }
func (m RecentDeploymentReferences) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m RecentDeploymentReferences) Less(i, j int) bool {
	return appsutil.DeploymentVersionFor(m[i].ReplicationController) > appsutil.DeploymentVersionFor(m[j].ReplicationController)
}

// TemplateImage is a structure for helping a caller iterate over a PodSpec
type TemplateImage struct {
	Image string

	Ref *imageapi.DockerImageReference

	From *corev1.ObjectReference

	Container *corev1.Container
}

// templateImageForContainer takes a container and returns a TemplateImage.
func templateImageForContainer(container *corev1.Container, triggerFn TriggeredByFunc) (TemplateImage, error) {
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
func TemplateImageForContainer(pod *corev1.PodSpec, triggerFn TriggeredByFunc, containerName string) (TemplateImage, error) {
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
func eachTemplateImage(container *corev1.Container, triggerFn TriggeredByFunc, fn func(TemplateImage, error)) {
	image, err := templateImageForContainer(container, triggerFn)
	fn(image, err)
}

// EachTemplateImage iterates a pod spec, looking for triggers that match each container and invoking
// fn with each located image.
func EachTemplateImage(pod *corev1.PodSpec, triggerFn TriggeredByFunc, fn func(TemplateImage, error)) {
	for i := range pod.Containers {
		eachTemplateImage(&pod.Containers[i], triggerFn, fn)
	}
	for i := range pod.InitContainers {
		eachTemplateImage(&pod.InitContainers[i], triggerFn, fn)
	}
}

// TriggeredByFunc returns a TemplateImage or error from the provided container
type TriggeredByFunc func(container *corev1.Container) (TemplateImage, bool)

// IgnoreTriggers ignores the triggers
func IgnoreTriggers(container *corev1.Container) (TemplateImage, bool) {
	return TemplateImage{}, false
}

// DeploymentConfigHasTrigger returns a function that can identify the image for each container.
func DeploymentConfigHasTrigger(config *appsv1.DeploymentConfig) TriggeredByFunc {
	return func(container *corev1.Container) (TemplateImage, bool) {
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
