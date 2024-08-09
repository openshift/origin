package watchresources

import (
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"sort"
	"strings"
)

func NodeRoles(node *v1.Node) string {
	const roleLabel = "node-role.kubernetes.io/"
	var roles []string
	for label := range node.Labels {
		if strings.Contains(label, roleLabel) {
			role := label[len(roleLabel):]
			if role == "" {
				logrus.Warningf("ignoring blank role label %s", roleLabel)
				continue
			}
			roles = append(roles, role)
		}
	}

	sort.Strings(roles)
	return strings.Join(roles, ",")
}

func FindNodeCondition(status []v1.NodeCondition, name v1.NodeConditionType, position int) *v1.NodeCondition {
	if position < len(status) {
		if status[position].Type == name {
			return &status[position]
		}
	}
	for i := range status {
		if status[i].Type == name {
			return &status[i]
		}
	}
	return nil
}
