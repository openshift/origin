package quota

import "github.com/openshift/api/quota/v1"

// ConvertClusterResourceQuotaToAppliedClusterQuota returns back a converted AppliedClusterResourceQuota which is NOT a deep copy.
func ConvertClusterResourceQuotaToAppliedClusterResourceQuota(in *ClusterResourceQuota) *AppliedClusterResourceQuota {
	return &AppliedClusterResourceQuota{
		ObjectMeta: in.ObjectMeta,
		Spec:       in.Spec,
		Status:     in.Status,
	}
}

// ConvertClusterResourceQuotaToAppliedClusterQuota returns back a converted AppliedClusterResourceQuota which is NOT a deep copy.
func ConvertAppliedClusterResourceQuotaToClusterResourceQuota(in *AppliedClusterResourceQuota) *ClusterResourceQuota {
	return &ClusterResourceQuota{
		ObjectMeta: in.ObjectMeta,
		Spec:       in.Spec,
		Status:     in.Status,
	}
}

// ConvertV1AppliedClusterResourceQuotaToV1ClusterResourceQuota returns back a converted AppliedClusterResourceQuota which is NOT a deep copy.
func ConvertV1AppliedClusterResourceQuotaToV1ClusterResourceQuota(in *v1.AppliedClusterResourceQuota) *v1.ClusterResourceQuota {
	return &v1.ClusterResourceQuota{
		ObjectMeta: in.ObjectMeta,
		Spec:       in.Spec,
		Status:     in.Status,
	}
}
