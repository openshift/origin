package quota

import (
	quotav1 "github.com/openshift/api/quota/v1"
)

// ConvertClusterResourceQuotaToAppliedClusterQuota returns back a converted AppliedClusterResourceQuota which is NOT a deep copy.
func ConvertAppliedClusterResourceQuotaToClusterResourceQuota(in *quotav1.AppliedClusterResourceQuota) *quotav1.ClusterResourceQuota {
	return &quotav1.ClusterResourceQuota{
		ObjectMeta: in.ObjectMeta,
		Spec:       in.Spec,
		Status:     in.Status,
	}
}

// ConvertClusterResourceQuotaToAppliedClusterQuota returns back a converted AppliedClusterResourceQuota which is NOT a deep copy.
func ConvertV1ClusterResourceQuotaToV1AppliedClusterResourceQuota(in *quotav1.ClusterResourceQuota) *quotav1.AppliedClusterResourceQuota {
	return &quotav1.AppliedClusterResourceQuota{
		ObjectMeta: in.ObjectMeta,
		Spec:       in.Spec,
		Status:     in.Status,
	}
}

// ConvertV1AppliedClusterResourceQuotaToV1ClusterResourceQuota returns back a converted AppliedClusterResourceQuota which is NOT a deep copy.
func ConvertV1AppliedClusterResourceQuotaToV1ClusterResourceQuota(in *quotav1.AppliedClusterResourceQuota) *quotav1.ClusterResourceQuota {
	return &quotav1.ClusterResourceQuota{
		ObjectMeta: in.ObjectMeta,
		Spec:       in.Spec,
		Status:     in.Status,
	}
}
