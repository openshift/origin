package hardcodedauthorizer

import (
	"context"

	"k8s.io/apiserver/pkg/authorization/authorizer"
)

type metricsAuthorizer struct{}

// GetUser() user.Info - checked
// GetVerb() string - checked
// IsReadOnly() bool - na
// GetNamespace() string - na
// GetResource() string - na
// GetSubresource() string - na
// GetName() string - na
// GetAPIGroup() string - na
// GetAPIVersion() string - na
// IsResourceRequest() bool - checked
// GetPath() string - checked
func (m metricsAuthorizer) Authorize(ctx context.Context, a authorizer.Attributes) (authorized authorizer.Decision, reason string, err error) {
	if a.GetUser().GetName() != "system:serviceaccount:openshift-monitoring:prometheus-k8s" {
		return authorizer.DecisionNoOpinion, "", nil
	}
	if !a.IsResourceRequest() &&
		a.GetVerb() == "get" &&
		a.GetPath() == "/metrics" {
		return authorizer.DecisionAllow, "requesting metrics is allowed", nil
	}

	return authorizer.DecisionNoOpinion, "", nil
}

func (m metricsAuthorizer) ConditionsAwareAuthorize(ctx context.Context, a authorizer.Attributes) authorizer.ConditionsAwareDecision {
	return authorizer.ConditionsAwareDecisionFromParts(m.Authorize(ctx, a))
}

func (m metricsAuthorizer) EvaluateConditions(_ context.Context, _ authorizer.ConditionsAwareDecision, _ authorizer.ConditionsData) (authorizer.Decision, string, error) {
	return authorizer.DecisionDeny, "", authorizer.ErrorConditionEvaluationNotSupported
}

// NewHardCodedMetricsAuthorizer returns a hardcoded authorizer for checking metrics.
func NewHardCodedMetricsAuthorizer() *metricsAuthorizer {
	return new(metricsAuthorizer)
}
