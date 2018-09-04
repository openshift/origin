package kubegraph

import (
	"encoding/json"
	"strings"

	"github.com/gonum/graph"

	kappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	oapps "github.com/openshift/api/apps"
	appsv1 "github.com/openshift/api/apps/v1"
	"github.com/openshift/origin/pkg/api/legacy"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	triggerapi "github.com/openshift/origin/pkg/image/apis/image/v1/trigger"
	"github.com/openshift/origin/pkg/image/trigger/annotations"
	"github.com/openshift/origin/pkg/oc/lib/graph/appsgraph"
	appsnodes "github.com/openshift/origin/pkg/oc/lib/graph/appsgraph/nodes"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
	imagegraph "github.com/openshift/origin/pkg/oc/lib/graph/imagegraph/nodes"
	kubegraph "github.com/openshift/origin/pkg/oc/lib/graph/kubegraph/nodes"
)

const (
	// ExposedThroughServiceEdgeKind goes from a PodTemplateSpec or a Pod to Service.  The head should make the service's selector.
	ExposedThroughServiceEdgeKind = "ExposedThroughService"
	// MountedSecretEdgeKind goes from PodSpec to Secret indicating that is or will be a request to mount a volume with the Secret.
	MountedSecretEdgeKind = "MountedSecret"
	// MountableSecretEdgeKind goes from ServiceAccount to Secret indicating that the SA allows the Secret to be mounted
	MountableSecretEdgeKind = "MountableSecret"
	// ReferencedServiceAccountEdgeKind goes from PodSpec to ServiceAccount indicating that Pod is or will be running as the SA.
	ReferencedServiceAccountEdgeKind = "ReferencedServiceAccount"
	// ScalingEdgeKind goes from HorizontalPodAutoscaler to scaled objects indicating that the HPA scales the object
	ScalingEdgeKind = "Scaling"
	// TriggersDeploymentEdgeKind points from DeploymentConfigs to ImageStreamTags that trigger the deployment
	TriggersDeploymentEdgeKind = "TriggersDeployment"
	// UsedInDeploymentEdgeKind points from DeploymentConfigs to DockerImageReferences that are used in the deployment
	UsedInDeploymentEdgeKind = "UsedInDeployment"
	// DeploymentEdgeKind points from Deployment to the ReplicaSet that are fulfilling the deployment
	DeploymentEdgeKind = "Deployment"
)

// AddExposedPodTemplateSpecEdges ensures that a directed edge exists between a service and all the PodTemplateSpecs
// in the graph that match the service selector
func AddExposedPodTemplateSpecEdges(g osgraph.MutableUniqueGraph, node *kubegraph.ServiceNode) {
	if node.Service.Spec.Selector == nil {
		return
	}
	query := labels.SelectorFromSet(node.Service.Spec.Selector)
	for _, n := range g.(graph.Graph).Nodes() {
		switch target := n.(type) {
		case *kubegraph.PodTemplateSpecNode:
			if target.Namespace != node.Namespace {
				continue
			}

			if query.Matches(labels.Set(target.PodTemplateSpec.Labels)) {
				g.AddEdge(target, node, ExposedThroughServiceEdgeKind)
			}
		}
	}
}

// AddAllExposedPodTemplateSpecEdges calls AddExposedPodTemplateSpecEdges for every ServiceNode in the graph
func AddAllExposedPodTemplateSpecEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).Nodes() {
		if serviceNode, ok := node.(*kubegraph.ServiceNode); ok {
			AddExposedPodTemplateSpecEdges(g, serviceNode)
		}
	}
}

// AddExposedPodEdges ensures that a directed edge exists between a service and all the pods
// in the graph that match the service selector
func AddExposedPodEdges(g osgraph.MutableUniqueGraph, node *kubegraph.ServiceNode) {
	if node.Service.Spec.Selector == nil {
		return
	}
	query := labels.SelectorFromSet(node.Service.Spec.Selector)
	for _, n := range g.(graph.Graph).Nodes() {
		switch target := n.(type) {
		case *kubegraph.PodNode:
			if target.Namespace != node.Namespace {
				continue
			}
			if query.Matches(labels.Set(target.Labels)) {
				g.AddEdge(target, node, ExposedThroughServiceEdgeKind)
			}
		}
	}
}

// AddAllExposedPodEdges calls AddExposedPodEdges for every ServiceNode in the graph
func AddAllExposedPodEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).Nodes() {
		if serviceNode, ok := node.(*kubegraph.ServiceNode); ok {
			AddExposedPodEdges(g, serviceNode)
		}
	}
}

// AddManagedByControllerPodEdges ensures that a directed edge exists between a controller and all the pods
// in the graph that match the label selector
func AddManagedByControllerPodEdges(g osgraph.MutableUniqueGraph, to graph.Node, namespace string, selector map[string]string) {
	if selector == nil {
		return
	}
	query := labels.SelectorFromSet(selector)
	for _, n := range g.(graph.Graph).Nodes() {
		switch target := n.(type) {
		case *kubegraph.PodNode:
			if target.Namespace != namespace {
				continue
			}
			if query.Matches(labels.Set(target.Labels)) {
				g.AddEdge(target, to, appsgraph.ManagedByControllerEdgeKind)
			}
		}
	}
}

// AddAllManagedByControllerPodEdges calls AddManagedByControllerPodEdges for every node in the graph
// TODO: should do this through an interface (selects pods)
func AddAllManagedByControllerPodEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).Nodes() {
		switch cast := node.(type) {
		case *kubegraph.ReplicationControllerNode:
			AddManagedByControllerPodEdges(g, cast, cast.ReplicationController.Namespace, cast.ReplicationController.Spec.Selector)
		case *kubegraph.ReplicaSetNode:
			selector := make(map[string]string)
			if cast.ReplicaSet.Spec.Selector != nil {
				selector = cast.ReplicaSet.Spec.Selector.MatchLabels
			}
			AddManagedByControllerPodEdges(g, cast, cast.ReplicaSet.Namespace, selector)
		case *kubegraph.StatefulSetNode:
			selector := make(map[string]string)
			if cast.StatefulSet.Spec.Selector != nil {
				selector = cast.StatefulSet.Spec.Selector.MatchLabels
			}
			AddManagedByControllerPodEdges(g, cast, cast.StatefulSet.Namespace, selector)
		case *kubegraph.DaemonSetNode:
			selector := make(map[string]string)
			if cast.DaemonSet.Spec.Selector != nil {
				selector = cast.DaemonSet.Spec.Selector.MatchLabels
			}
			AddManagedByControllerPodEdges(g, cast, cast.DaemonSet.Namespace, selector)
		}
	}
}

func AddMountedSecretEdges(g osgraph.Graph, podSpec *kubegraph.PodSpecNode) {
	//pod specs are always contained.  We'll get the toplevel container so that we can pull a namespace from it
	containerNode := osgraph.GetTopLevelContainerNode(g, podSpec)
	containerObj := g.GraphDescriber.Object(containerNode)

	meta, err := meta.Accessor(containerObj.(runtime.Object))
	if err != nil {
		// this should never happen.  it means that a podSpec is owned by a top level container that is not a runtime.Object
		panic(err)
	}

	for _, volume := range podSpec.Volumes {
		source := volume.VolumeSource
		if source.Secret == nil {
			continue
		}

		// pod secrets must be in the same namespace
		syntheticSecret := &corev1.Secret{}
		syntheticSecret.Namespace = meta.GetNamespace()
		syntheticSecret.Name = source.Secret.SecretName

		secretNode := kubegraph.FindOrCreateSyntheticSecretNode(g, syntheticSecret)
		g.AddEdge(podSpec, secretNode, MountedSecretEdgeKind)
	}
}

func AddAllMountedSecretEdges(g osgraph.Graph) {
	for _, node := range g.Nodes() {
		if podSpecNode, ok := node.(*kubegraph.PodSpecNode); ok {
			AddMountedSecretEdges(g, podSpecNode)
		}
	}
}

func AddMountableSecretEdges(g osgraph.Graph, saNode *kubegraph.ServiceAccountNode) {
	for _, mountableSecret := range saNode.ServiceAccount.Secrets {
		syntheticSecret := &corev1.Secret{}
		syntheticSecret.Namespace = saNode.ServiceAccount.Namespace
		syntheticSecret.Name = mountableSecret.Name

		secretNode := kubegraph.FindOrCreateSyntheticSecretNode(g, syntheticSecret)
		g.AddEdge(saNode, secretNode, MountableSecretEdgeKind)
	}
}

func AddAllMountableSecretEdges(g osgraph.Graph) {
	for _, node := range g.Nodes() {
		if saNode, ok := node.(*kubegraph.ServiceAccountNode); ok {
			AddMountableSecretEdges(g, saNode)
		}
	}
}

func AddRequestedServiceAccountEdges(g osgraph.Graph, podSpecNode *kubegraph.PodSpecNode) {
	//pod specs are always contained.  We'll get the toplevel container so that we can pull a namespace from it
	containerNode := osgraph.GetTopLevelContainerNode(g, podSpecNode)
	containerObj := g.GraphDescriber.Object(containerNode)

	meta, err := meta.Accessor(containerObj.(runtime.Object))
	if err != nil {
		panic(err)
	}

	// if no SA name is present, admission will set 'default'
	name := "default"
	if len(podSpecNode.ServiceAccountName) > 0 {
		name = podSpecNode.ServiceAccountName
	}

	syntheticSA := &corev1.ServiceAccount{}
	syntheticSA.Namespace = meta.GetNamespace()
	syntheticSA.Name = name

	saNode := kubegraph.FindOrCreateSyntheticServiceAccountNode(g, syntheticSA)
	g.AddEdge(podSpecNode, saNode, ReferencedServiceAccountEdgeKind)
}

func AddAllRequestedServiceAccountEdges(g osgraph.Graph) {
	for _, node := range g.Nodes() {
		if podSpecNode, ok := node.(*kubegraph.PodSpecNode); ok {
			AddRequestedServiceAccountEdges(g, podSpecNode)
		}
	}
}

func AddHPAScaleRefEdges(g osgraph.Graph, restMapper meta.RESTMapper) {
	for _, node := range g.NodesByKind(kubegraph.HorizontalPodAutoscalerNodeKind) {
		hpaNode := node.(*kubegraph.HorizontalPodAutoscalerNode)

		syntheticMeta := metav1.ObjectMeta{
			Name:      hpaNode.HorizontalPodAutoscaler.Spec.ScaleTargetRef.Name,
			Namespace: hpaNode.HorizontalPodAutoscaler.Namespace,
		}

		var groupVersionResource schema.GroupVersionResource
		resource := strings.ToLower(hpaNode.HorizontalPodAutoscaler.Spec.ScaleTargetRef.Kind)
		if groupVersion, err := schema.ParseGroupVersion(hpaNode.HorizontalPodAutoscaler.Spec.ScaleTargetRef.APIVersion); err == nil {
			groupVersionResource = groupVersion.WithResource(resource)
		} else {
			groupVersionResource = schema.GroupVersionResource{Resource: resource}
		}

		groupVersionResource, err := restMapper.ResourceFor(groupVersionResource)
		if err != nil {
			continue
		}

		var syntheticNode graph.Node
		r := groupVersionResource.GroupResource()
		switch r {
		case corev1.Resource("replicationcontrollers"):
			syntheticNode = kubegraph.FindOrCreateSyntheticReplicationControllerNode(g, &corev1.ReplicationController{ObjectMeta: syntheticMeta})
		case oapps.Resource("deploymentconfigs"),
			// we need the legacy resource until we stop supporting HPA having old refs
			legacy.Resource("deploymentconfigs"):
			syntheticNode = appsnodes.FindOrCreateSyntheticDeploymentConfigNode(g, &appsv1.DeploymentConfig{ObjectMeta: syntheticMeta})
		case kappsv1.Resource("deployments"):
			syntheticNode = kubegraph.FindOrCreateSyntheticDeploymentNode(g, &kappsv1.Deployment{ObjectMeta: syntheticMeta})
		case kappsv1.Resource("replicasets"):
			syntheticNode = kubegraph.FindOrCreateSyntheticReplicaSetNode(g, &kappsv1.ReplicaSet{ObjectMeta: syntheticMeta})
		default:
			continue
		}

		g.AddEdge(hpaNode, syntheticNode, ScalingEdgeKind)
	}
}

func addTriggerEdges(obj runtime.Object, podTemplate corev1.PodTemplateSpec, addEdgeFn func(image appsgraph.TemplateImage, err error)) {
	m, err := meta.Accessor(obj)
	if err != nil {
		return
	}
	triggerAnnotation, ok := m.GetAnnotations()[triggerapi.TriggerAnnotationKey]
	if !ok {
		return
	}
	triggers := []triggerapi.ObjectFieldTrigger{}
	if err := json.Unmarshal([]byte(triggerAnnotation), &triggers); err != nil {
		return
	}
	triggerFn := func(container *corev1.Container) (appsgraph.TemplateImage, bool) {
		from := corev1.ObjectReference{}
		for _, trigger := range triggers {
			c, remainder, err := annotations.ContainerForObjectFieldPath(obj, trigger.FieldPath)
			if err != nil || remainder != "image" {
				continue
			}
			from.Namespace = trigger.From.Namespace
			if len(from.Namespace) == 0 {
				from.Namespace = m.GetNamespace()
			}
			from.Name = trigger.From.Name
			from.Kind = trigger.From.Kind
			if len(from.Kind) == 0 {
				from.Kind = "ImageStreamTag"
			}
			return appsgraph.TemplateImage{
				Image: c.GetImage(),
				From:  &from,
			}, true
		}
		return appsgraph.TemplateImage{}, false
	}
	appsgraph.EachTemplateImage(&podTemplate.Spec, triggerFn, addEdgeFn)
}

func AddTriggerStatefulSetsEdges(g osgraph.MutableUniqueGraph, node *kubegraph.StatefulSetNode) *kubegraph.StatefulSetNode {
	addTriggerEdges(node.StatefulSet, node.StatefulSet.Spec.Template, func(image appsgraph.TemplateImage, err error) {
		if err != nil {
			return
		}
		if image.From != nil {
			if len(image.From.Name) == 0 {
				return
			}
			name, tag, _ := imageapi.SplitImageStreamTag(image.From.Name)
			in := imagegraph.FindOrCreateSyntheticImageStreamTagNode(g, imagegraph.MakeImageStreamTagObjectMeta(image.From.Namespace, name, tag))
			g.AddEdge(in, node, TriggersDeploymentEdgeKind)
			return
		}

		tag := image.Ref.Tag
		image.Ref.Tag = ""
		in := imagegraph.EnsureDockerRepositoryNode(g, image.Ref.String(), tag)
		g.AddEdge(in, node, UsedInDeploymentEdgeKind)
	})
	return node
}

func AddAllTriggerStatefulSetsEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).Nodes() {
		if sNode, ok := node.(*kubegraph.StatefulSetNode); ok {
			AddTriggerStatefulSetsEdges(g, sNode)
		}
	}
}

func AddTriggerDeploymentsEdges(g osgraph.MutableUniqueGraph, node *kubegraph.DeploymentNode) *kubegraph.DeploymentNode {
	addTriggerEdges(node.Deployment, node.Deployment.Spec.Template, func(image appsgraph.TemplateImage, err error) {
		if err != nil {
			return
		}
		if image.From != nil {
			if len(image.From.Name) == 0 {
				return
			}
			name, tag, _ := imageapi.SplitImageStreamTag(image.From.Name)
			in := imagegraph.FindOrCreateSyntheticImageStreamTagNode(g, imagegraph.MakeImageStreamTagObjectMeta(image.From.Namespace, name, tag))
			g.AddEdge(in, node, TriggersDeploymentEdgeKind)
			return
		}
		tag := image.Ref.Tag
		image.Ref.Tag = ""
		in := imagegraph.EnsureDockerRepositoryNode(g, image.Ref.String(), tag)
		g.AddEdge(in, node, UsedInDeploymentEdgeKind)
	})
	return node
}

func AddAllTriggerDeploymentsEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).Nodes() {
		if dNode, ok := node.(*kubegraph.DeploymentNode); ok {
			AddTriggerDeploymentsEdges(g, dNode)
		}
	}
}

func AddDeploymentEdges(g osgraph.MutableUniqueGraph, node *kubegraph.DeploymentNode) *kubegraph.DeploymentNode {
	for _, n := range g.(graph.Graph).Nodes() {
		if rsNode, ok := n.(*kubegraph.ReplicaSetNode); ok {
			if rsNode.ReplicaSet.Namespace != node.Deployment.Namespace {
				continue
			}
			if BelongsToDeployment(node.Deployment, rsNode.ReplicaSet) {
				g.AddEdge(node, rsNode, DeploymentEdgeKind)
				g.AddEdge(rsNode, node, appsgraph.ManagedByControllerEdgeKind)
			}
		}
	}

	return node
}

func BelongsToDeployment(config *kappsv1.Deployment, b *kappsv1.ReplicaSet) bool {
	if b.OwnerReferences == nil {
		return false
	}
	for _, ref := range b.OwnerReferences {
		if ref.Kind == "Deployment" && ref.Name == config.Name {
			return true
		}
	}
	return false
}

func AddAllDeploymentEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).Nodes() {
		if dNode, ok := node.(*kubegraph.DeploymentNode); ok {
			AddDeploymentEdges(g, dNode)
		}
	}
}
