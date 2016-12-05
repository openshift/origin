package api

import "k8s.io/kubernetes/pkg/conversion"

func DeepCopy_api_ResourceQuotasStatusByNamespace(in ResourceQuotasStatusByNamespace, out *ResourceQuotasStatusByNamespace, c *conversion.Cloner) error {
	*out = in.DeepCopy()
	return nil
}
