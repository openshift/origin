package auth

import (
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"

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

// Reviewer performs access reviews for a project by name
type Reviewer interface {
	Review(name string) (Review, error)
}

type authorizerReviewer struct {
	policyChecker rbac.SubjectLocator
}

func NewAuthorizerReviewer(policyChecker rbac.SubjectLocator) Reviewer {
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

	subjects, err := r.policyChecker.AllowedSubjects(attributes)
	review := &defaultReview{}
	review.users, review.groups = authorizationutil.RBACSubjectsToUsersAndGroups(subjects, attributes.GetNamespace())
	if err != nil {
		review.evaluationError = err.Error()
	}
	return review, nil
}
