package lifecycle

import "k8s.io/apimachinery/pkg/runtime/schema"

func AccessReviewResources() map[schema.GroupResource]bool {
	return accessReviewResources
}
