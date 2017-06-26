package util

import (
	restclient "k8s.io/client-go/rest"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/client"
)

func CanRequestProjects(config *restclient.Config, defaultNamespace string) (bool, error) {
	oClient, err := client.New(config)
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

	listResponse, err := oClient.SubjectAccessReviews().Create(sar)
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

	createResponse, err := oClient.SubjectAccessReviews().Create(sar)
	if err != nil {
		return false, err
	}

	return (listResponse.Allowed && createResponse.Allowed), nil
}
