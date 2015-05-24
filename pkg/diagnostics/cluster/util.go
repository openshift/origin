package cluster

import (
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	osclient "github.com/openshift/origin/pkg/client"
)

func adminCan(client *osclient.Client, ns string, sar *authorizationapi.SubjectAccessReview) (bool, error) {
	if resp, err := client.SubjectAccessReviews(ns).Create(sar); err != nil {
		return false, err
	} else if resp.Allowed {
		return true, nil
	}
	return false, nil
}
