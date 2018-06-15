package appsgraph

import (
	"github.com/golang/glog"
	"github.com/gonum/graph"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	appsgraph "github.com/openshift/origin/pkg/oc/graph/appsgraph/nodes"
	osgraph "github.com/openshift/origin/pkg/oc/graph/genericgraph"
	imagegraph "github.com/openshift/origin/pkg/oc/graph/imagegraph/nodes"
	kubeedges "github.com/openshift/origin/pkg/oc/graph/kubegraph"
	kubegraph "github.com/openshift/origin/pkg/oc/graph/kubegraph/nodes"
)

const (
	// TriggersDeploymentEdgeKind points from DeploymentConfigs to ImageStreamTags that trigger the deployment
	TriggersDeploymentEdgeKind = "TriggersDeployment"
	// UsedInDeploymentEdgeKind points from DeploymentConfigs to DockerImageReferences that are used in the deployment
	UsedInDeploymentEdgeKind = "UsedInDeployment"
	// DeploymentEdgeKind points from DeploymentConfigs to the ReplicationControllers that are fulfilling the deployment
	DeploymentEdgeKind = "Deployment"
	// VolumeClaimEdgeKind goes from DeploymentConfigs to PersistentVolumeClaims indicating a request for persistent storage.
	VolumeClaimEdgeKind = "VolumeClaim"
)

// AddTriggerDeploymentConfigsEdges creates edges that point to named Docker image repositories for each image used in the deployment.
func AddTriggerDeploymentConfigsEdges(g osgraph.MutableUniqueGraph, node *appsgraph.DeploymentConfigNode) *appsgraph.DeploymentConfigNode {
	podTemplate := node.DeploymentConfig.Spec.Template
	if podTemplate == nil {
		return node
	}

	appsapi.EachTemplateImage(
		&podTemplate.Spec,
		appsapi.DeploymentConfigHasTrigger(node.DeploymentConfig),
		func(image appsapi.TemplateImage, err error) {
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

func AddAllTriggerDeploymentConfigsEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).Nodes() {
		if dcNode, ok := node.(*appsgraph.DeploymentConfigNode); ok {
			AddTriggerDeploymentConfigsEdges(g, dcNode)
		}
	}
}

func AddDeploymentConfigsDeploymentEdges(g osgraph.MutableUniqueGraph, node *appsgraph.DeploymentConfigNode) *appsgraph.DeploymentConfigNode {
	for _, n := range g.(graph.Graph).Nodes() {
		if rcNode, ok := n.(*kubegraph.ReplicationControllerNode); ok {
			if rcNode.ReplicationController.Namespace != node.DeploymentConfig.Namespace {
				continue
			}
			if BelongsToDeploymentConfig(node.DeploymentConfig, rcNode.ReplicationController) {
				g.AddEdge(node, rcNode, DeploymentEdgeKind)
				g.AddEdge(rcNode, node, kubeedges.ManagedByControllerEdgeKind)
			}
		}
	}

	return node
}

func AddAllDeploymentConfigsDeploymentEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).Nodes() {
		if dcNode, ok := node.(*appsgraph.DeploymentConfigNode); ok {
			AddDeploymentConfigsDeploymentEdges(g, dcNode)
		}
	}
}

func AddVolumeClaimEdges(g osgraph.Graph, dcNode *appsgraph.DeploymentConfigNode) {
	podTemplate := dcNode.DeploymentConfig.Spec.Template
	if podTemplate == nil {
		glog.Warningf("DeploymentConfig %s/%s template should not be empty", dcNode.DeploymentConfig.Namespace, dcNode.DeploymentConfig.Name)
		return
	}
	for _, volume := range podTemplate.Spec.Volumes {
		source := volume.VolumeSource
		if source.PersistentVolumeClaim == nil {
			continue
		}

		syntheticClaim := &kapi.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      source.PersistentVolumeClaim.ClaimName,
				Namespace: dcNode.DeploymentConfig.Namespace,
			},
		}

		pvcNode := kubegraph.FindOrCreateSyntheticPVCNode(g, syntheticClaim)
		// TODO: Consider direction
		g.AddEdge(dcNode, pvcNode, VolumeClaimEdgeKind)
	}
}

func AddAllVolumeClaimEdges(g osgraph.Graph) {
	for _, node := range g.Nodes() {
		if dcNode, ok := node.(*appsgraph.DeploymentConfigNode); ok && dcNode.IsFound {
			AddVolumeClaimEdges(g, dcNode)
		}
	}
}
