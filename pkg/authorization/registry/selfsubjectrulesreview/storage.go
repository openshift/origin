package selfsubjectrulesreview

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type REST struct {
	ruleResolver rulevalidation.AuthorizationRuleResolver
}

func NewREST(ruleResolver rulevalidation.AuthorizationRuleResolver) *REST {
	return &REST{ruleResolver: ruleResolver}
}

func (r *REST) New() runtime.Object {
	return &authorizationapi.SelfSubjectRulesReview{}
}

// Create registers a given new ResourceAccessReview instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	// the input object has no valuable input, so don't bother checking it.false
	policyRules, err := r.ruleResolver.GetEffectivePolicyRules(ctx)

	ret := &authorizationapi.SelfSubjectRulesReview{
		Status: authorizationapi.SubjectRulesReviewStatus{
			Rules: policyRules,
		},
	}

	if err != nil {
		ret.Status.EvaluationError = err.Error()
	}

	return ret, nil
}
