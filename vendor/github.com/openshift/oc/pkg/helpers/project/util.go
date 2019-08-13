package project

import (
	restclient "k8s.io/client-go/rest"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	authorizationv1typedclient "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
)

func CanRequestProjects(config *restclient.Config, defaultNamespace string) (bool, error) {
	oClient, err := authorizationv1typedclient.NewForConfig(config)
	if err != nil {
		return false, err
	}

	sar := &authorizationv1.SubjectAccessReview{
		Action: authorizationv1.Action{
			Namespace: defaultNamespace,
			Verb:      "list",
			Resource:  "projectrequests",
		},
	}

	listResponse, err := oClient.SubjectAccessReviews().Create(sar)
	if err != nil {
		return false, err
	}

	sar = &authorizationv1.SubjectAccessReview{
		Action: authorizationv1.Action{
			Namespace: defaultNamespace,
			Verb:      "create",
			Resource:  "projectrequests",
		},
	}

	createResponse, err := oClient.SubjectAccessReviews().Create(sar)
	if err != nil {
		return false, err
	}

	return listResponse.Allowed && createResponse.Allowed, nil
}
