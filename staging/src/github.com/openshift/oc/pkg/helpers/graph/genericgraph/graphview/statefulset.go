package graphview

import (
	"github.com/openshift/origin/pkg/oc/lib/graph/appsgraph"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
	"github.com/openshift/origin/pkg/oc/lib/graph/kubegraph"
	kubenodes "github.com/openshift/origin/pkg/oc/lib/graph/kubegraph/nodes"
)

type StatefulSet struct {
	StatefulSet *kubenodes.StatefulSetNode

	OwnedPods   []*kubenodes.PodNode
	CreatedPods []*kubenodes.PodNode

	Images []ImagePipeline

	// TODO: handle conflicting once controller refs are present, not worth it yet
}

// AllStatefulSets returns all the StatefulSets that aren't in the excludes set and the set of covered NodeIDs
func AllStatefulSets(g osgraph.Graph, excludeNodeIDs IntSet) ([]StatefulSet, IntSet) {
	covered := IntSet{}
	views := []StatefulSet{}

	for _, uncastNode := range g.NodesByKind(kubenodes.StatefulSetNodeKind) {
		if excludeNodeIDs.Has(uncastNode.ID()) {
			continue
		}

		view, covers := NewStatefulSet(g, uncastNode.(*kubenodes.StatefulSetNode))
		covered.Insert(covers.List()...)
		views = append(views, view)
	}

	return views, covered
}

// NewStatefulSet returns the StatefulSet and a set of all the NodeIDs covered by the StatefulSet
func NewStatefulSet(g osgraph.Graph, node *kubenodes.StatefulSetNode) (StatefulSet, IntSet) {
	covered := IntSet{}
	covered.Insert(node.ID())

	view := StatefulSet{}
	view.StatefulSet = node

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
