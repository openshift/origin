package analysis

import (
	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubeedges "github.com/openshift/origin/pkg/api/kubegraph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
)

// CheckMountedSecrets checks to be sure that all the referenced secrets are mountable (by service account) and present (not synthetic)
func CheckMountedSecrets(g osgraph.Graph, podSpecNode *kubegraph.PodSpecNode) ( /*unmountable secrets*/ []*kubegraph.SecretNode /*unresolved secrets*/, []*kubegraph.SecretNode) {
	saNodes := g.SuccessorNodesByNodeAndEdgeKind(podSpecNode, kubegraph.ServiceAccountNodeKind, kubeedges.ReferencedServiceAccountEdgeKind)
	saMountableSecrets := []*kubegraph.SecretNode{}

	if len(saNodes) > 0 {
		saNode := saNodes[0].(*kubegraph.ServiceAccountNode)
		for _, secretNode := range g.SuccessorNodesByNodeAndEdgeKind(saNode, kubegraph.SecretNodeKind, kubeedges.MountableSecretEdgeKind) {
			saMountableSecrets = append(saMountableSecrets, secretNode.(*kubegraph.SecretNode))
		}
	}

	unmountableSecrets := []*kubegraph.SecretNode{}
	missingSecrets := []*kubegraph.SecretNode{}

	for _, uncastMountedSecretNode := range g.SuccessorNodesByNodeAndEdgeKind(podSpecNode, kubegraph.SecretNodeKind, kubeedges.MountedSecretEdgeKind) {
		mountedSecretNode := uncastMountedSecretNode.(*kubegraph.SecretNode)
		if !mountedSecretNode.Found() {
			missingSecrets = append(missingSecrets, mountedSecretNode)
		}

		mountable := false
		for _, mountableSecretNode := range saMountableSecrets {
			if mountableSecretNode == mountedSecretNode {
				mountable = true
				break
			}
		}

		if !mountable {
			unmountableSecrets = append(unmountableSecrets, mountedSecretNode)
			continue
		}
	}

	return unmountableSecrets, missingSecrets
}
