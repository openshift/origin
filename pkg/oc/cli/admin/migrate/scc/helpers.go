package scc

import (
	"fmt"

	"k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func convertCapabilities(inputCapabilities []v1.Capability) []v1.Capability {
	caps := make([]v1.Capability, 0, len(inputCapabilities))
	for _, capability := range inputCapabilities {
		v1Cap := v1.Capability(string(capability))
		caps = append(caps, v1Cap)
	}
	return caps
}

func int64val(value *int64) string {
	if value == nil {
		return "nil"
	}

	return fmt.Sprintf("%d", *value)
}

func createSingleIDRange(min, max int64) []policy.IDRange {
	ranges := make([]policy.IDRange, 1)
	ranges[0] = policy.IDRange{
		Min: min,
		Max: max,
	}
	return ranges
}

func collectSubjectNames(subjects []rbacv1.Subject, accept func(rbacv1.Subject) bool) []string {
	result := []string{}
	for _, subject := range subjects {
		if accept(subject) {
			result = append(result, subject.Name)
		}
	}
	return result
}
