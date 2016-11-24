package graphview

import (
	"sort"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	"github.com/openshift/origin/pkg/api/kubegraph/analysis"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
)

type DeploymentPipeline struct {
	Deployment *kubegraph.DeploymentNode

	ActiveDeployment    *kubegraph.ReplicaSetNode
	InactiveDeployments []*kubegraph.ReplicaSetNode
}

// AllDeploymentPipelines returns all the DCPipelines that aren't in the excludes set and the set of covered NodeIDs
func AllDeploymentPipelines(g osgraph.Graph, excludeNodeIDs IntSet) ([]DeploymentPipeline, IntSet) {
	covered := IntSet{}
	dcPipelines := []DeploymentPipeline{}

	for _, uncastNode := range g.NodesByKind(kubegraph.DeploymentNodeKind) {
		if excludeNodeIDs.Has(uncastNode.ID()) {
			continue
		}

		pipeline, covers := NewDeploymentPipeline(g, uncastNode.(*kubegraph.DeploymentNode))
		covered.Insert(covers.List()...)
		dcPipelines = append(dcPipelines, pipeline)
	}

	sort.Sort(SortedDeploymentPipeline(dcPipelines))
	return dcPipelines, covered
}

// NewDeploymentPipeline returns the DeploymentPipeline and a set of all the NodeIDs covered by the DeploymentPipeline
func NewDeploymentPipeline(g osgraph.Graph, dcNode *kubegraph.DeploymentNode) (DeploymentPipeline, IntSet) {
	covered := IntSet{}
	covered.Insert(dcNode.ID())

	dcPipeline := DeploymentPipeline{}
	dcPipeline.Deployment = dcNode

	dcPipeline.ActiveDeployment, dcPipeline.InactiveDeployments = analysis.RelevantDeployments(g, dcNode)
	for _, rc := range dcPipeline.InactiveDeployments {
		_, covers := NewReplicaSet(g, rc)
		covered.Insert(covers.List()...)
	}

	if dcPipeline.ActiveDeployment != nil {
		_, covers := NewReplicaSet(g, dcPipeline.ActiveDeployment)
		covered.Insert(covers.List()...)
	}

	return dcPipeline, covered
}

type SortedDeploymentPipeline []DeploymentPipeline

func (m SortedDeploymentPipeline) Len() int      { return len(m) }
func (m SortedDeploymentPipeline) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m SortedDeploymentPipeline) Less(i, j int) bool {
	return CompareObjectMeta(&m[i].Deployment.Deployment.ObjectMeta, &m[j].Deployment.Deployment.ObjectMeta)
}
