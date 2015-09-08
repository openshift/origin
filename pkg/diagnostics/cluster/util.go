package cluster

import (
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	osclient "github.com/openshift/origin/pkg/client"
)

func adminCan(client *osclient.Client, action authorizationapi.AuthorizationAttributes) (bool, error) {
	if resp, err := client.SubjectAccessReviews().Create(&authorizationapi.SubjectAccessReview{Action: action}); err != nil {
		return false, err
	} else if resp.Allowed {
		return true, nil
	}
	return false, nil
}
