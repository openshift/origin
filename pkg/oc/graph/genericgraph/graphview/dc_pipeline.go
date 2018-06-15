package graphview

import (
	"sort"

	appsedges "github.com/openshift/origin/pkg/oc/graph/appsgraph"
	appsgraph "github.com/openshift/origin/pkg/oc/graph/appsgraph/nodes"
	osgraph "github.com/openshift/origin/pkg/oc/graph/genericgraph"
	kubegraph "github.com/openshift/origin/pkg/oc/graph/kubegraph/nodes"
)

type DeploymentConfigPipeline struct {
	DeploymentConfig *appsgraph.DeploymentConfigNode

	ActiveDeployment    *kubegraph.ReplicationControllerNode
	InactiveDeployments []*kubegraph.ReplicationControllerNode

	Images []ImagePipeline
}

// AllDeploymentConfigPipelines returns all the DCPipelines that aren't in the excludes set and the set of covered NodeIDs
func AllDeploymentConfigPipelines(g osgraph.Graph, excludeNodeIDs IntSet) ([]DeploymentConfigPipeline, IntSet) {
	covered := IntSet{}
	dcPipelines := []DeploymentConfigPipeline{}

	for _, uncastNode := range g.NodesByKind(appsgraph.DeploymentConfigNodeKind) {
		if excludeNodeIDs.Has(uncastNode.ID()) {
			continue
		}

		pipeline, covers := NewDeploymentConfigPipeline(g, uncastNode.(*appsgraph.DeploymentConfigNode))
		covered.Insert(covers.List()...)
		dcPipelines = append(dcPipelines, pipeline)
	}

	sort.Sort(SortedDeploymentConfigPipeline(dcPipelines))
	return dcPipelines, covered
}

// NewDeploymentConfigPipeline returns the DeploymentConfigPipeline and a set of all the NodeIDs covered by the DeploymentConfigPipeline
func NewDeploymentConfigPipeline(g osgraph.Graph, dcNode *appsgraph.DeploymentConfigNode) (DeploymentConfigPipeline, IntSet) {
	covered := IntSet{}
	covered.Insert(dcNode.ID())

	dcPipeline := DeploymentConfigPipeline{}
	dcPipeline.DeploymentConfig = dcNode

	// for everything that can trigger a deployment, create an image pipeline and add it to the list
	for _, istNode := range g.PredecessorNodesByEdgeKind(dcNode, appsedges.TriggersDeploymentEdgeKind) {
		imagePipeline, covers := NewImagePipelineFromImageTagLocation(g, istNode, istNode.(ImageTagLocation))

		covered.Insert(covers.List()...)
		dcPipeline.Images = append(dcPipeline.Images, imagePipeline)
	}

	// for image that we use, create an image pipeline and add it to the list
	for _, tagNode := range g.PredecessorNodesByEdgeKind(dcNode, appsedges.UsedInDeploymentEdgeKind) {
		imagePipeline, covers := NewImagePipelineFromImageTagLocation(g, tagNode, tagNode.(ImageTagLocation))

		covered.Insert(covers.List()...)
		dcPipeline.Images = append(dcPipeline.Images, imagePipeline)
	}

	dcPipeline.ActiveDeployment, dcPipeline.InactiveDeployments = appsedges.RelevantDeployments(g, dcNode)
	for _, rc := range dcPipeline.InactiveDeployments {
		_, covers := NewReplicationController(g, rc)
		covered.Insert(covers.List()...)
	}

	if dcPipeline.ActiveDeployment != nil {
		_, covers := NewReplicationController(g, dcPipeline.ActiveDeployment)
		covered.Insert(covers.List()...)
	}

	return dcPipeline, covered
}

type SortedDeploymentConfigPipeline []DeploymentConfigPipeline

func (m SortedDeploymentConfigPipeline) Len() int      { return len(m) }
func (m SortedDeploymentConfigPipeline) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m SortedDeploymentConfigPipeline) Less(i, j int) bool {
	return CompareObjectMeta(&m[i].DeploymentConfig.DeploymentConfig.ObjectMeta, &m[j].DeploymentConfig.DeploymentConfig.ObjectMeta)
}
