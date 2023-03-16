package storage

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/warning"
	api "k8s.io/kubernetes/pkg/apis/core"
)

// OpenShift 4.8 and 4.9 only - BZ 2045576
// In kube 1.21 and 1.22 (OCP 4.8 and 4.9), the apiserver would default the value of `ipFamilyPolicy` to `RequireDualStack`
// if you created a Service with two `ipFamilies` or two `clusterIPs` but no explicitly-specified `ipFamilyPolicy`.
// In 1.23 / 4.10, you MUST explicitly specify either "ipFamilyPolicy: PreferDualStack" or "ipFamilyPolicy: RequireDualStack"
// for the service to be valid.
// Emit a warning in 4.8 and 4.9 if such services are created or updated.
// Using a mutating or validating admission controller webhook for services is a big "no". Therefore, we implement this downstream
// only warning message via OpenShift's kube-apiserver.
// Carry this change forward only in 4.8 and 4.9 and drop this for all other versions.
func warnDualStackIPFamilyPolicy(ctx context.Context, service *api.Service) {
	if service.Spec.IPFamilyPolicy == nil && (len(service.Spec.IPFamilies) == 2 || len(service.Spec.ClusterIPs) == 2) {
		msg := field.NewPath("service", "spec", "ipFamilyPolicy").String() + " must be RequireDualStack or PreferDualStack when " +
			"multiple 'ipFamilies' are specified, this operation will fail starting with Red Hat OpenShift Platform 4.10."
		warning.AddWarning(ctx, "", msg)
	}
}
