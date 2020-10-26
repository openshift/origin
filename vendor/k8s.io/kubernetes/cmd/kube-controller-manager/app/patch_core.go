package app

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/quota/v1"
	"k8s.io/kubernetes/pkg/quota/v1/generic"

	imagev1 "github.com/openshift/api/image/v1"
)

var legacyObjectCountAliases = map[schema.GroupVersionResource]corev1.ResourceName{
	imagev1.GroupVersion.WithResource("imagestreams"): imagev1.ResourceImageStreams,
}

// openShiftResourceQuotaEvaluators returns OpenShift specific quota evaluators
func openShiftResourceQuotaEvaluators(listerFuncForResource quota.ListerForResourceFunc) []quota.Evaluator {
	result := []quota.Evaluator{}

	// these evaluators require an alias for backwards compatibility
	for gvr, alias := range legacyObjectCountAliases {
		result = append(result,
			generic.NewObjectCountEvaluator(gvr.GroupResource(), generic.ListResourceUsingListerFunc(listerFuncForResource, gvr), alias))
	}

	return result
}
