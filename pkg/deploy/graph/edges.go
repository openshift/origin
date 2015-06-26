package graph

import (
	"sort"

	"github.com/gonum/graph"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	deploygraph "github.com/openshift/origin/pkg/deploy/graph/nodes"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
)

const (
	TriggersDeploymentEdgeKind = "TriggersDeployment"
	UsedInDeploymentEdgeKind   = "UsedInDeployment"
)

// AddTriggerEdges creates edges that point to named Docker image repositories for each image used in the deployment.
func AddTriggerEdges(g osgraph.MutableUniqueGraph, node *deploygraph.DeploymentConfigNode) *deploygraph.DeploymentConfigNode {
	rcTemplate := node.DeploymentConfig.Template.ControllerTemplate.Template
	if rcTemplate == nil {
		return node
	}

	EachTemplateImage(
		&rcTemplate.Spec,
		DeploymentConfigHasTrigger(node.DeploymentConfig),
		func(image TemplateImage, err error) {
			if err != nil {
				return
			}
			if image.From != nil {
				if len(image.From.Name) == 0 {
					return
				}

				in := imagegraph.FindOrCreateSyntheticImageStreamTagNode(g, imagegraph.MakeImageStreamTagObjectMeta(image.From.Namespace, image.From.Name, image.FromTag))
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

func AddAllTriggerEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).NodeList() {
		if dcNode, ok := node.(*deploygraph.DeploymentConfigNode); ok {
			AddTriggerEdges(g, dcNode)
		}
	}
}

// TODO kill this.  It should be based on an edge traversal to loaded replication controllers
func JoinDeployments(node *deploygraph.DeploymentConfigNode, deploys []kapi.ReplicationController) {
	matches := []*kapi.ReplicationController{}
	for i := range deploys {
		if belongsToDeploymentConfig(node.DeploymentConfig, &deploys[i]) {
			matches = append(matches, &deploys[i])
		}
	}
	if len(matches) == 0 {
		return
	}
	sort.Sort(RecentDeploymentReferences(matches))
	if node.DeploymentConfig.LatestVersion == deployutil.DeploymentVersionFor(matches[0]) {
		node.ActiveDeployment = matches[0]
		node.Deployments = matches[1:]
		return
	}
	node.Deployments = matches
}
