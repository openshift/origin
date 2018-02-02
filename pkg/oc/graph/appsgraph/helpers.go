package appsgraph

import (
	"sort"

	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	appsgraph "github.com/openshift/origin/pkg/oc/graph/appsgraph/nodes"
	osgraph "github.com/openshift/origin/pkg/oc/graph/genericgraph"
	kubegraph "github.com/openshift/origin/pkg/oc/graph/kubegraph/nodes"
)

// RelevantDeployments returns the active deployment and a list of inactive deployments (in order from newest to oldest)
func RelevantDeployments(g osgraph.Graph, dcNode *appsgraph.DeploymentConfigNode) (*kubegraph.ReplicationControllerNode, []*kubegraph.ReplicationControllerNode) {
	allDeployments := []*kubegraph.ReplicationControllerNode{}
	uncastDeployments := g.SuccessorNodesByEdgeKind(dcNode, DeploymentEdgeKind)
	if len(uncastDeployments) == 0 {
		return nil, []*kubegraph.ReplicationControllerNode{}
	}

	for i := range uncastDeployments {
		allDeployments = append(allDeployments, uncastDeployments[i].(*kubegraph.ReplicationControllerNode))
	}

	sort.Sort(RecentDeploymentReferences(allDeployments))

	if dcNode.DeploymentConfig.Status.LatestVersion == appsutil.DeploymentVersionFor(allDeployments[0].ReplicationController) {
		return allDeployments[0], allDeployments[1:]
	}

	return nil, allDeployments
}

func BelongsToDeploymentConfig(config *appsapi.DeploymentConfig, b *kapi.ReplicationController) bool {
	if b.Annotations != nil {
		return config.Name == appsutil.DeploymentConfigNameFor(b)
	}
	return false
}

type RecentDeploymentReferences []*kubegraph.ReplicationControllerNode

func (m RecentDeploymentReferences) Len() int      { return len(m) }
func (m RecentDeploymentReferences) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m RecentDeploymentReferences) Less(i, j int) bool {
	return appsutil.DeploymentVersionFor(m[i].ReplicationController) > appsutil.DeploymentVersionFor(m[j].ReplicationController)
}
