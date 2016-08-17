package analysis

import (
	"sort"
	"strconv"

	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/util/deployment"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
)

// Revision returns the revision number of the input replica set
// TODO: Use the upstream helper when it will be available
func revision(rs *extensions.ReplicaSet) int64 {
	v, ok := rs.Annotations[deployment.RevisionAnnotation]
	if !ok {
		return 0
	}
	rev, _ := strconv.ParseInt(v, 10, 64)
	return rev
}

// RelevantDeployments returns the active deployment and a list of inactive deployments (in order from newest to oldest)
func RelevantDeployments(g osgraph.Graph, dNode *kubegraph.DeploymentNode) (*kubegraph.ReplicaSetNode, []*kubegraph.ReplicaSetNode) {
	allDeployments := []*kubegraph.ReplicaSetNode{}
	uncastDeployments := g.SuccessorNodesByEdgeKind(dNode, "Deployment")
	if len(uncastDeployments) == 0 {
		return nil, []*kubegraph.ReplicaSetNode{}
	}

	for i := range uncastDeployments {
		allDeployments = append(allDeployments, uncastDeployments[i].(*kubegraph.ReplicaSetNode))
	}

	sort.Sort(RecentDeploymentReferences(allDeployments))

	latestVersion, _ := strconv.ParseInt(dNode.Deployment.Annotations[deployment.RevisionAnnotation], 10, 64)
	if latestVersion == revision(allDeployments[0].ReplicaSet) {
		return allDeployments[0], allDeployments[1:]
	}

	return nil, allDeployments
}

type RecentDeploymentReferences []*kubegraph.ReplicaSetNode

func (m RecentDeploymentReferences) Len() int      { return len(m) }
func (m RecentDeploymentReferences) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m RecentDeploymentReferences) Less(i, j int) bool {
	return revision(m[i].ReplicaSet) > revision(m[j].ReplicaSet)
}
