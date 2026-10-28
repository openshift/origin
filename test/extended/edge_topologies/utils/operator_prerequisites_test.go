package utils

import (
	"testing"

	v1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsRecoveryEssentialOperator(t *testing.T) {
	for _, name := range RecoveryEssentialOperators {
		if !isRecoveryEssentialOperator(name) {
			t.Errorf("expected %q to be essential", name)
		}
	}
	nonEssential := []string{"image-registry", "console", "monitoring", "marketplace", "authentication", "ingress", "config-operator"}
	for _, name := range nonEssential {
		if isRecoveryEssentialOperator(name) {
			t.Errorf("expected %q to NOT be essential", name)
		}
	}
}

func TestNonEssentialOperatorStatus_HasFailures(t *testing.T) {
	tests := []struct {
		name     string
		status   NonEssentialOperatorStatus
		expected bool
	}{
		{
			name:     "no unhealthy operators",
			status:   NonEssentialOperatorStatus{Healthy: 5, Total: 5},
			expected: false,
		},
		{
			name: "one unhealthy operator",
			status: NonEssentialOperatorStatus{
				Unhealthy: []UnhealthyOperator{{Name: "image-registry", Available: false}},
				Healthy:   4,
				Total:     5,
			},
			expected: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.status.HasFailures(); got != tc.expected {
				t.Errorf("HasFailures() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestNonEssentialOperatorStatus_Summary(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		s := NonEssentialOperatorStatus{Healthy: 3, Total: 3}
		got := s.Summary()
		if got != "all 3 non-essential operators healthy" {
			t.Errorf("unexpected summary: %q", got)
		}
	})

	t.Run("unhealthy includes condition details", func(t *testing.T) {
		s := NonEssentialOperatorStatus{
			Unhealthy: []UnhealthyOperator{
				{
					Name:      "image-registry",
					Available: false,
					Degraded:  false,
					Conditions: []v1.ClusterOperatorStatusCondition{
						{
							Type:               v1.OperatorAvailable,
							Status:             v1.ConditionFalse,
							Reason:             "NoReplicasAvailable",
							Message:            "no replicas available",
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			Healthy: 4,
			Total:   5,
		}
		summary := s.Summary()
		if len(summary) == 0 {
			t.Fatal("expected non-empty summary")
		}
		for _, needle := range []string{
			"image-registry",
			"Available=false",
			"NoReplicasAvailable",
			"no replicas available",
			"1 of 5",
		} {
			if !contains(summary, needle) {
				t.Errorf("summary missing %q:\n%s", needle, summary)
			}
		}
	})
}

func TestOperatorNamespace(t *testing.T) {
	tests := []struct {
		operator string
		expected string
	}{
		{"image-registry", "openshift-image-registry"},
		{"console", "openshift-console"},
		{"monitoring", "openshift-monitoring"},
		{"unknown-operator", ""},
	}
	for _, tc := range tests {
		t.Run(tc.operator, func(t *testing.T) {
			if got := operatorNamespace(tc.operator); got != tc.expected {
				t.Errorf("operatorNamespace(%q) = %q, want %q", tc.operator, got, tc.expected)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
