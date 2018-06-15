package util

import (
	restclient "k8s.io/client-go/rest"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
)

func CanRequestProjects(config *restclient.Config, defaultNamespace string) (bool, error) {
	oClient, err := authorizationclient.NewForConfig(config)
	if err != nil {
		return false, err
	}

	sar := &authorizationapi.SubjectAccessReview{
		Action: authorizationapi.Action{
			Namespace: defaultNamespace,
			Verb:      "list",
			Resource:  "projectrequests",
		},
	}

	listResponse, err := oClient.Authorization().SubjectAccessReviews().Create(sar)
	if err != nil {
		return false, err
	}

	sar = &authorizationapi.SubjectAccessReview{
		Action: authorizationapi.Action{
			Namespace: defaultNamespace,
			Verb:      "create",
			Resource:  "projectrequests",
		},
	}

	createResponse, err := oClient.Authorization().SubjectAccessReviews().Create(sar)
	if err != nil {
		return false, err
	}

	return (listResponse.Allowed && createResponse.Allowed), nil
}
