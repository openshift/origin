package nodes

import (
	"github.com/gonum/graph"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	depoyapi "github.com/openshift/origin/pkg/deploy/api"
)

// EnsureDeploymentConfigNode adds the provided deployment config to the graph if it does not exist
func EnsureDeploymentConfigNode(g osgraph.MutableUniqueGraph, config *depoyapi.DeploymentConfig) *DeploymentConfigNode {
	return osgraph.EnsureUnique(
		g,
		DeploymentConfigNodeName(config),
		func(node osgraph.Node) graph.Node {
			return &DeploymentConfigNode{Node: node, DeploymentConfig: config}
		},
	).(*DeploymentConfigNode)
}
