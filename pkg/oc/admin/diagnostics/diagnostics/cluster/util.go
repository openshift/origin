package cluster

import (
	"k8s.io/kubernetes/pkg/apis/authorization"
	authorizationtypedclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"
)

func userCan(sarClient authorizationtypedclient.SelfSubjectAccessReviewsGetter, action *authorization.ResourceAttributes) (bool, error) {
	resp, err := sarClient.SelfSubjectAccessReviews().Create(&authorization.SelfSubjectAccessReview{
		Spec: authorization.SelfSubjectAccessReviewSpec{
			ResourceAttributes: action,
		},
	})
	if err != nil {
		return false, err
	}

	if resp.Status.Allowed {
		return true, nil
	}

	return false, nil
}
