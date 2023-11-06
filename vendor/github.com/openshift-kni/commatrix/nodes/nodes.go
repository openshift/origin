package nodes

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/openshift-kni/commatrix/consts"
)

func GetRole(node *corev1.Node) string {
	if _, ok := node.Labels[consts.RoleLabel+"master"]; ok {
		return "master"
	}

	if _, ok := node.Labels[consts.RoleLabel+"worker"]; ok {
		return "worker"
	}

	return ""
}
