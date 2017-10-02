package auth

import (
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	authorizerrbac "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationutil "github.com/openshift/origin/pkg/authorization/util"
)

// Review is a list of users and groups that can access a resource
type Review interface {
	Users() []string
	Groups() []string
	EvaluationError() string
}

type defaultReview struct {
	users           []string
	groups          []string
	evaluationError string
}

func (r *defaultReview) Users() []string {
	return r.users
}

// Groups returns the groups that can access a resource
func (r *defaultReview) Groups() []string {
	return r.groups
}

func (r *defaultReview) EvaluationError() string {
	return r.evaluationError
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

func (r *review) EvaluationError() string {
	return r.response.EvaluationError
}

// Reviewer performs access reviews for a project by name
type Reviewer interface {
	Review(name string) (Review, error)
}

type authorizerReviewer struct {
	policyChecker authorizerrbac.SubjectLocator
}

func NewAuthorizerReviewer(policyChecker authorizerrbac.SubjectLocator) Reviewer {
	return &authorizerReviewer{policyChecker: policyChecker}
}

func (r *authorizerReviewer) Review(namespaceName string) (Review, error) {
	attributes := kauthorizer.AttributesRecord{
		Verb:            "get",
		Namespace:       namespaceName,
		Resource:        "namespaces",
		Name:            namespaceName,
		ResourceRequest: true,
	}

	// err is non fatal and partial success is possible
	subjects, err := r.policyChecker.AllowedSubjects(attributes)
	users, groups, expandSubjectErrors := authorizationutil.ExpandSubjects(namespaceName, subjects)

	if expandSubjectErrors != nil {
		return nil, kerrors.NewAggregate(expandSubjectErrors)
	}

	review := &defaultReview{
		users:  users.List(),
		groups: groups.List(),
	}
	if err != nil {
		review.evaluationError = err.Error()
	}
	return review, nil
}
