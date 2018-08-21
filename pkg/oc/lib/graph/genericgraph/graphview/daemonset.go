package graphview

import (
	"github.com/openshift/origin/pkg/oc/lib/graph/appsgraph"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
	"github.com/openshift/origin/pkg/oc/lib/graph/kubegraph"
	kubenodes "github.com/openshift/origin/pkg/oc/lib/graph/kubegraph/nodes"
)

type DaemonSet struct {
	DaemonSet *kubenodes.DaemonSetNode

	OwnedPods   []*kubenodes.PodNode
	CreatedPods []*kubenodes.PodNode

	Images []ImagePipeline
}

// AllDaemonSets returns all the DaemonSets that aren't in the excludes set and the set of covered NodeIDs
func AllDaemonSets(g osgraph.Graph, excludeNodeIDs IntSet) ([]DaemonSet, IntSet) {
	covered := IntSet{}
	views := []DaemonSet{}

	for _, uncastNode := range g.NodesByKind(kubenodes.DaemonSetNodeKind) {
		if excludeNodeIDs.Has(uncastNode.ID()) {
			continue
		}

		view, covers := NewDaemonSet(g, uncastNode.(*kubenodes.DaemonSetNode))
		covered.Insert(covers.List()...)
		views = append(views, view)
	}

	return views, covered
}

// NewDaemonSet returns the DaemonSet and a set of all the NodeIDs covered by the DaemonSet
func NewDaemonSet(g osgraph.Graph, node *kubenodes.DaemonSetNode) (DaemonSet, IntSet) {
	covered := IntSet{}
	covered.Insert(node.ID())

	view := DaemonSet{}
	view.DaemonSet = node

	for _, uncastPodNode := range g.PredecessorNodesByEdgeKind(node, appsgraph.ManagedByControllerEdgeKind) {
		podNode := uncastPodNode.(*kubenodes.PodNode)
		covered.Insert(podNode.ID())
		view.OwnedPods = append(view.OwnedPods, podNode)
	}

	for _, istNode := range g.PredecessorNodesByEdgeKind(node, kubegraph.TriggersDeploymentEdgeKind) {
		imagePipeline, covers := NewImagePipelineFromImageTagLocation(g, istNode, istNode.(ImageTagLocation))
		covered.Insert(covers.List()...)
		view.Images = append(view.Images, imagePipeline)
	}

	// for image that we use, create an image pipeline and add it to the list
	for _, tagNode := range g.PredecessorNodesByEdgeKind(node, appsgraph.UsedInDeploymentEdgeKind) {
		imagePipeline, covers := NewImagePipelineFromImageTagLocation(g, tagNode, tagNode.(ImageTagLocation))

		covered.Insert(covers.List()...)
		view.Images = append(view.Images, imagePipeline)
	}

	return view, covered
}
