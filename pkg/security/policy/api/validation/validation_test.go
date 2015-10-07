package validation

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	errors "k8s.io/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/security/policy/api"
)

func TestValidatePodSecurityPolicy(t *testing.T) {
	var invalidUID int64 = -1
	errorCases := map[string]struct {
		scc         *api.PodSecurityPolicy
		errorType   errors.ValidationErrorType
		errorDetail string
	}{
		"no user options": {
			scc: &api.PodSecurityPolicy{
				ObjectMeta: kapi.ObjectMeta{Name: "foo"},
				Spec: api.PodSecurityPolicySpec{
					SELinuxContext: api.SELinuxContextStrategyOptions{
						Type: api.SELinuxStrategyMustRunAs,
					},
				},
			},
			errorType:   errors.ValidationErrorTypeInvalid,
			errorDetail: "invalid strategy type; valid values are MustRunAs, MustRunAsNonRoot, RunAsAny",
		},
		"no selinux options": {
			scc: &api.PodSecurityPolicy{
				ObjectMeta: kapi.ObjectMeta{Name: "foo"},
				Spec: api.PodSecurityPolicySpec{
					RunAsUser: api.RunAsUserStrategyOptions{
						Type: api.RunAsUserStrategyMustRunAs,
					},
				},
			},
			errorType:   errors.ValidationErrorTypeInvalid,
			errorDetail: "invalid strategy type; valid values are MustRunAs, RunAsAny",
		},
		"invalid user strategy type": {
			scc: &api.PodSecurityPolicy{
				ObjectMeta: kapi.ObjectMeta{Name: "foo"},
				Spec: api.PodSecurityPolicySpec{
					RunAsUser: api.RunAsUserStrategyOptions{
						Type: "invalid",
					},
					SELinuxContext: api.SELinuxContextStrategyOptions{
						Type: api.SELinuxStrategyMustRunAs,
					},
				},
			},
			errorType:   errors.ValidationErrorTypeInvalid,
			errorDetail: "invalid strategy type; valid values are MustRunAs, MustRunAsNonRoot, RunAsAny",
		},
		"invalid selinux strategy type": {
			scc: &api.PodSecurityPolicy{
				ObjectMeta: kapi.ObjectMeta{Name: "foo"},
				Spec: api.PodSecurityPolicySpec{
					RunAsUser: api.RunAsUserStrategyOptions{
						Type: api.RunAsUserStrategyMustRunAs,
					},
					SELinuxContext: api.SELinuxContextStrategyOptions{
						Type: "invalid",
					},
				},
			},
			errorType:   errors.ValidationErrorTypeInvalid,
			errorDetail: "invalid strategy type; valid values are MustRunAs, RunAsAny",
		},
		"invalid uid": {
			scc: &api.PodSecurityPolicy{
				ObjectMeta: kapi.ObjectMeta{Name: "foo"},
				Spec: api.PodSecurityPolicySpec{
					RunAsUser: api.RunAsUserStrategyOptions{
						Type: api.RunAsUserStrategyMustRunAs,
						UID:  &invalidUID,
					},
					SELinuxContext: api.SELinuxContextStrategyOptions{
						Type: api.SELinuxStrategyMustRunAs,
					},
				},
			},
			errorType:   errors.ValidationErrorTypeInvalid,
			errorDetail: "uid cannot be negative",
		},
		"missing object meta name": {
			scc: &api.PodSecurityPolicy{
				Spec: api.PodSecurityPolicySpec{
					RunAsUser: api.RunAsUserStrategyOptions{
						Type: api.RunAsUserStrategyMustRunAs,
					},
					SELinuxContext: api.SELinuxContextStrategyOptions{
						Type: api.SELinuxStrategyMustRunAs,
					},
				},
			},
			errorType:   errors.ValidationErrorTypeRequired,
			errorDetail: "name or generateName is required",
		},
	}

	for k, v := range errorCases {
		if errs := ValidatePodSecurityPolicy(v.scc); len(errs) == 0 || errs[0].(*errors.ValidationError).Type != v.errorType || errs[0].(*errors.ValidationError).Detail != v.errorDetail {
			t.Errorf("Expected error type %s with detail %s for %s, got %v", v.errorType, v.errorDetail, k, errs)
		}
	}

	var validUID int64 = 1
	successCases := map[string]struct {
		scc *api.PodSecurityPolicy
	}{
		"must run as": {
			scc: &api.PodSecurityPolicy{
				ObjectMeta: kapi.ObjectMeta{Name: "foo"},
				Spec: api.PodSecurityPolicySpec{
					RunAsUser: api.RunAsUserStrategyOptions{
						Type: api.RunAsUserStrategyMustRunAs,
						UID:  &validUID,
					},
					SELinuxContext: api.SELinuxContextStrategyOptions{
						Type: api.SELinuxStrategyMustRunAs,
					},
				},
			},
		},
		"run as any": {
			scc: &api.PodSecurityPolicy{
				ObjectMeta: kapi.ObjectMeta{Name: "foo"},
				Spec: api.PodSecurityPolicySpec{
					RunAsUser: api.RunAsUserStrategyOptions{
						Type: api.RunAsUserStrategyRunAsAny,
					},
					SELinuxContext: api.SELinuxContextStrategyOptions{
						Type: api.SELinuxStrategyRunAsAny,
					},
				},
			},
		},
		"run as non-root (user only)": {
			scc: &api.PodSecurityPolicy{
				ObjectMeta: kapi.ObjectMeta{Name: "foo"},
				Spec: api.PodSecurityPolicySpec{
					RunAsUser: api.RunAsUserStrategyOptions{
						Type: api.RunAsUserStrategyMustRunAsNonRoot,
					},
					SELinuxContext: api.SELinuxContextStrategyOptions{
						Type: api.SELinuxStrategyRunAsAny,
					},
				},
			},
		},
	}

	for k, v := range successCases {
		if errs := ValidatePodSecurityPolicy(v.scc); len(errs) != 0 {
			t.Errorf("Expected success for %s, got %v", k, errs)
		}
	}
}
