package remote

import (
	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	authzapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	oclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

// RemoteAuthorizer provides authorization using subject access review and resource access review requests
type RemoteAuthorizer struct {
	client RemoteAuthorizerClient
}

type RemoteAuthorizerClient interface {
	oclient.SubjectAccessReviews
	oclient.ResourceAccessReviews

	oclient.LocalSubjectAccessReviewsNamespacer
	oclient.LocalResourceAccessReviewsNamespacer
}

func NewAuthorizer(client RemoteAuthorizerClient) (authorizer.Authorizer, error) {
	return &RemoteAuthorizer{client}, nil
}

func (r *RemoteAuthorizer) Authorize(ctx kapi.Context, a authorizer.Action) (bool, string, error) {
	var (
		result *authzapi.SubjectAccessReviewResponse
		err    error
	)

	// Extract namespace from context
	namespace, _ := kapi.NamespaceFrom(ctx)

	// Extract user from context
	user := ""
	groups := sets.NewString()
	userInfo, ok := kapi.UserFrom(ctx)
	if ok {
		user = userInfo.GetName()
		groups.Insert(userInfo.GetGroups()...)
	}

	// Make sure we don't run a subject access review on our own permissions
	if len(user) == 0 && len(groups) == 0 {
		user = bootstrappolicy.UnauthenticatedUsername
		groups = sets.NewString(bootstrappolicy.UnauthenticatedGroup)
	}

	if len(namespace) > 0 {
		result, err = r.client.LocalSubjectAccessReviews(namespace).Create(
			authzapi.AddUserToLSAR(userInfo, &authzapi.LocalSubjectAccessReview{Action: getAction(namespace, a)}))
	} else {
		result, err = r.client.SubjectAccessReviews().Create(
			authzapi.AddUserToSAR(userInfo, &authzapi.SubjectAccessReview{Action: getAction(namespace, a)}))
	}

	if err != nil {
		glog.Errorf("error running subject access review: %v", err)
		return false, "", kerrs.NewInternalError(err)
	}
	glog.V(2).Infof("allowed=%v, reason=%s", result.Allowed, result.Reason)
	return result.Allowed, result.Reason, nil
}

func (r *RemoteAuthorizer) GetAllowedSubjects(ctx kapi.Context, attributes authorizer.Action) (sets.String, sets.String, error) {
	var (
		result *authzapi.ResourceAccessReviewResponse
		err    error
	)

	// Extract namespace from context
	namespace, _ := kapi.NamespaceFrom(ctx)

	if len(namespace) > 0 {
		result, err = r.client.LocalResourceAccessReviews(namespace).Create(&authzapi.LocalResourceAccessReview{Action: getAction(namespace, attributes)})
	} else {
		result, err = r.client.ResourceAccessReviews().Create(&authzapi.ResourceAccessReview{Action: getAction(namespace, attributes)})
	}

	if err != nil {
		glog.Errorf("error running resource access review: %v", err)
		return nil, nil, kerrs.NewInternalError(err)
	}
	return result.Users, result.Groups, nil
}

func getAction(namespace string, attributes authorizer.Action) authzapi.Action {
	return authzapi.Action{
		Namespace:    namespace,
		Verb:         attributes.GetVerb(),
		Group:        attributes.GetAPIGroup(),
		Version:      attributes.GetAPIVersion(),
		Resource:     attributes.GetResource(),
		ResourceName: attributes.GetResourceName(),

		// TODO: missing from authorizer.Action:
		// Content

		// TODO: missing from authzapi.Action
		// RequestAttributes (unserializable?)
		// IsNonResourceURL
		// URL (doesn't make sense for remote authz?)
	}
}
