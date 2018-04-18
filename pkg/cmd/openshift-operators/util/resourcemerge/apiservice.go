package resourcemerge

import (
	"k8s.io/apimachinery/pkg/api/equality"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
)

func EnsureAPIService(modified *bool, existing *apiregistrationv1beta1.APIService, required apiregistrationv1beta1.APIService) {
	EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	// we stomp everything
	if !equality.Semantic.DeepEqual(existing.Spec, required.Spec) {
		*modified = true
		existing.Spec = required.Spec
	}
}
