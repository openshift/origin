package graphview

import (
	appsedges "github.com/openshift/origin/pkg/oc/graph/appsgraph"
	osgraph "github.com/openshift/origin/pkg/oc/graph/genericgraph"
	kubeedges "github.com/openshift/origin/pkg/oc/graph/kubegraph"
	kubegraph "github.com/openshift/origin/pkg/oc/graph/kubegraph/nodes"
)

type Deployment struct {
	Deployment *kubegraph.DeploymentNode

	ActiveDeployment    *kubegraph.ReplicaSetNode
	InactiveDeployments []*kubegraph.ReplicaSetNode

	Images []ImagePipeline

	// TODO: handle conflicting once controller refs are present, not worth it yet
}

// AllDeployments returns all the Deployments that aren't in the excludes set and the set of covered NodeIDs
func AllDeployments(g osgraph.Graph, excludeNodeIDs IntSet) ([]Deployment, IntSet) {
	covered := IntSet{}
	views := []Deployment{}

	for _, uncastNode := range g.NodesByKind(kubegraph.DeploymentNodeKind) {
		if excludeNodeIDs.Has(uncastNode.ID()) {
			continue
		}

		view, covers := NewDeployment(g, uncastNode.(*kubegraph.DeploymentNode))
		covered.Insert(covers.List()...)
		views = append(views, view)
	}

	return views, covered
}

// NewDeployment returns the Deployment and a set of all the NodeIDs covered by the Deployment
func NewDeployment(g osgraph.Graph, node *kubegraph.DeploymentNode) (Deployment, IntSet) {
	covered := IntSet{}
	covered.Insert(node.ID())

	view := Deployment{}
	view.Deployment = node

	for _, istNode := range g.PredecessorNodesByEdgeKind(node, kubeedges.TriggersDeploymentEdgeKind) {
		imagePipeline, covers := NewImagePipelineFromImageTagLocation(g, istNode, istNode.(ImageTagLocation))
		covered.Insert(covers.List()...)
		view.Images = append(view.Images, imagePipeline)
	}

	// for image that we use, create an image pipeline and add it to the list
	for _, tagNode := range g.PredecessorNodesByEdgeKind(node, appsedges.UsedInDeploymentEdgeKind) {
		imagePipeline, covers := NewImagePipelineFromImageTagLocation(g, tagNode, tagNode.(ImageTagLocation))

		covered.Insert(covers.List()...)
		view.Images = append(view.Images, imagePipeline)
	}

	view.ActiveDeployment, view.InactiveDeployments = kubeedges.RelevantDeployments(g, view.Deployment)
	for _, rs := range view.InactiveDeployments {
		_, covers := NewReplicaSet(g, rs)
		covered.Insert(covers.List()...)
	}

	if view.ActiveDeployment != nil {
		_, covers := NewReplicaSet(g, view.ActiveDeployment)
		covered.Insert(covers.List()...)
	}

	return view, covered
}
