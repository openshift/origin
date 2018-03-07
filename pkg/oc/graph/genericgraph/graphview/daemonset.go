package graphview

import (
	appsedges "github.com/openshift/origin/pkg/oc/graph/appsgraph"
	osgraph "github.com/openshift/origin/pkg/oc/graph/genericgraph"
	kubeedges "github.com/openshift/origin/pkg/oc/graph/kubegraph"
	kubegraph "github.com/openshift/origin/pkg/oc/graph/kubegraph/nodes"
)

type DaemonSet struct {
	DaemonSet *kubegraph.DaemonSetNode

	OwnedPods   []*kubegraph.PodNode
	CreatedPods []*kubegraph.PodNode

	Images []ImagePipeline
}

// AllDaemonSets returns all the DaemonSets that aren't in the excludes set and the set of covered NodeIDs
func AllDaemonSets(g osgraph.Graph, excludeNodeIDs IntSet) ([]DaemonSet, IntSet) {
	covered := IntSet{}
	views := []DaemonSet{}

	for _, uncastNode := range g.NodesByKind(kubegraph.DaemonSetNodeKind) {
		if excludeNodeIDs.Has(uncastNode.ID()) {
			continue
		}

		view, covers := NewDaemonSet(g, uncastNode.(*kubegraph.DaemonSetNode))
		covered.Insert(covers.List()...)
		views = append(views, view)
	}

	return views, covered
}

// NewDaemonSet returns the DaemonSet and a set of all the NodeIDs covered by the DaemonSet
func NewDaemonSet(g osgraph.Graph, node *kubegraph.DaemonSetNode) (DaemonSet, IntSet) {
	covered := IntSet{}
	covered.Insert(node.ID())

	view := DaemonSet{}
	view.DaemonSet = node

	for _, uncastPodNode := range g.PredecessorNodesByEdgeKind(node, kubeedges.ManagedByControllerEdgeKind) {
		podNode := uncastPodNode.(*kubegraph.PodNode)
		covered.Insert(podNode.ID())
		view.OwnedPods = append(view.OwnedPods, podNode)
	}

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

	return view, covered
}
