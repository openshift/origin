package cluster

import (
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	osclient "github.com/openshift/origin/pkg/client"
)

func userCan(sarClient osclient.SubjectAccessReviews, action authorizationapi.Action) (bool, error) {
	resp, err := sarClient.SubjectAccessReviews().Create(&authorizationapi.SubjectAccessReview{Action: action})
	if err != nil {
		return false, err
	}

	if resp.Allowed {
		return true, nil
	}

	return false, nil
}
