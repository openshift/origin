package nodes

import (
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/liornoy/node-comm-lib/pkg/consts"
)

func GetRoles(node *corev1.Node) string {
	res := ""
	// Filter out user-defined roles.
	validRoles := map[string]bool{"worker": true, "master": true}

	for label := range node.Labels {
		// Look for node-role label and extract role.
		if after, found := strings.CutPrefix(label, consts.RoleLabel); found {
			if !validRoles[after] {
				continue
			}

			res = after
			break
		}
	}

	return res
}
