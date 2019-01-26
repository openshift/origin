package create

import "k8s.io/apimachinery/pkg/runtime/schema"

func init() {
	specialVerbs["use"] = append(
		specialVerbs["use"],
		schema.GroupResource{
			Group:    "security.openshift.io",
			Resource: "securitycontextconstraints",
		},
	)
}
