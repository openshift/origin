package app

import (
	"context"

	"github.com/openshift/library-go/pkg/authorization/hardcodedauthorizer"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// wrapAuthorizerWithMetricsScraper add an authorizer to always approver the openshift metrics scraper.
// This eliminates an unnecessary SAR for scraping metrics and enables metrics gathering when network access
// to the kube-apiserver is interrupted
func wrapAuthorizerWithMetricsScraper(authz authorizer.UnconditionalAuthorizer) authorizer.UnconditionalAuthorizer {
	metrics := hardcodedauthorizer.NewHardCodedMetricsAuthorizer()
	return authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
		if decision, reason, err := metrics.Authorize(ctx, a); err != nil || decision == authorizer.DecisionAllow {
			return decision, reason, err
		}
		return authz.Authorize(ctx, a)
	})
}
