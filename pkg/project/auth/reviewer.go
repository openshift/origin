package auth

import (
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/client"
)

// Review is a list of users and groups that can access a resource
type Review interface {
	Users() []string
	Groups() []string
}

type defaultReview struct {
	users  []string
	groups []string
}

func (r *defaultReview) Users() []string {
	return r.users
}

// Groups returns the groups that can access a resource
func (r *defaultReview) Groups() []string {
	return r.groups
}

type review struct {
	response *authorizationapi.ResourceAccessReviewResponse
}

// Users returns the users that can access a resource
func (r *review) Users() []string {
	return r.response.Users.List()
}

// Groups returns the groups that can access a resource
func (r *review) Groups() []string {
	return r.response.Groups.List()
}

// Reviewer performs access reviews for a project by name
type Reviewer interface {
	Review(name string) (Review, error)
}

// reviewer performs access reviews for a project by name
type reviewer struct {
	resourceAccessReviewsNamespacer client.LocalResourceAccessReviewsNamespacer
}

// NewReviewer knows how to make access control reviews for a resource by name
func NewReviewer(resourceAccessReviewsNamespacer client.LocalResourceAccessReviewsNamespacer) Reviewer {
	return &reviewer{
		resourceAccessReviewsNamespacer: resourceAccessReviewsNamespacer,
	}
}

// Review performs a resource access review for the given resource by name
func (r *reviewer) Review(name string) (Review, error) {
	resourceAccessReview := &authorizationapi.LocalResourceAccessReview{
		Action: authorizationapi.AuthorizationAttributes{
			Verb:         "get",
			Resource:     "namespaces",
			ResourceName: name,
		},
	}

	response, err := r.resourceAccessReviewsNamespacer.LocalResourceAccessReviews(name).Create(resourceAccessReview)

	if err != nil {
		return nil, err
	}
	review := &review{
		response: response,
	}
	return review, nil
}

type authorizerReviewer struct {
	policyChecker authorizer.Authorizer
}

func NewAuthorizerReviewer(policyChecker authorizer.Authorizer) Reviewer {
	return &authorizerReviewer{policyChecker: policyChecker}
}

func (r *authorizerReviewer) Review(namespaceName string) (Review, error) {
	attributes := authorizer.DefaultAuthorizationAttributes{
		Verb:         "get",
		Resource:     "namespaces",
		ResourceName: namespaceName,
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), namespaceName)
	users, groups, err := r.policyChecker.GetAllowedSubjects(ctx, attributes)
	if err != nil {
		return nil, err
	}

	review := &defaultReview{
		users:  users.List(),
		groups: groups.List(),
	}
	return review, nil
}
