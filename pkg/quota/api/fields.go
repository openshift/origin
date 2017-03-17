package api

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/registry/generic"
)

func ClusterResourceQuotaToSelectableFields(quota *ClusterResourceQuota) fields.Set {
	return generic.ObjectMetaFieldsSet(&quota.ObjectMeta, false)
}
