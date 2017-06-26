package quota

import "k8s.io/apimachinery/pkg/fields"

func ClusterResourceQuotaToSelectableFields(quota *ClusterResourceQuota) fields.Set {
	return fields.Set{
		"metadata.name": quota.Name,
	}
}
