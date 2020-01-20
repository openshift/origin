package v1

import (
	"reflect"
	"testing"
)

func TestFeatureBuilder(t *testing.T) {
	tests := []struct {
		name     string
		actual   *FeatureGateEnabledDisabled
		expected *FeatureGateEnabledDisabled
	}{
		{
			name:     "nothing",
			actual:   newDefaultFeatures().toFeatures(),
			expected: defaultFeatures,
		},
		{
			name:   "disable-existing",
			actual: newDefaultFeatures().without("SCTPSupport").toFeatures(),
			expected: &FeatureGateEnabledDisabled{
				Enabled: []string{
					"RotateKubeletServerCertificate",
					"SupportPodPidsLimit",
					"NodeDisruptionExclusion",
					"ServiceNodeExclusion",
					"IPv6DualStack",
				},
				Disabled: []string{
					"LegacyNodeRoleBehavior",
					"SCTPSupport",
				},
			},
		},
		{
			name:   "enable-existing",
			actual: newDefaultFeatures().with("LegacyNodeRoleBehavior").toFeatures(),
			expected: &FeatureGateEnabledDisabled{
				Enabled: []string{
					"RotateKubeletServerCertificate",
					"SupportPodPidsLimit",
					"NodeDisruptionExclusion",
					"ServiceNodeExclusion",
					"SCTPSupport",
					"IPv6DualStack",
					"LegacyNodeRoleBehavior",
				},
				Disabled: []string{},
			},
		},
		{
			name:   "disable-more",
			actual: newDefaultFeatures().without("SCTPSupport", "other").toFeatures(),
			expected: &FeatureGateEnabledDisabled{
				Enabled: []string{
					"RotateKubeletServerCertificate",
					"SupportPodPidsLimit",
					"NodeDisruptionExclusion",
					"ServiceNodeExclusion",
					"IPv6DualStack",
				},
				Disabled: []string{
					"LegacyNodeRoleBehavior",
					"SCTPSupport",
					"other",
				},
			},
		},
		{
			name:   "enable-more",
			actual: newDefaultFeatures().with("LegacyNodeRoleBehavior", "other").toFeatures(),
			expected: &FeatureGateEnabledDisabled{
				Enabled: []string{
					"RotateKubeletServerCertificate",
					"SupportPodPidsLimit",
					"NodeDisruptionExclusion",
					"ServiceNodeExclusion",
					"SCTPSupport",
					"IPv6DualStack",
					"LegacyNodeRoleBehavior",
					"other",
				},
				Disabled: []string{},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !reflect.DeepEqual(tc.expected, tc.actual) {
				t.Error(tc.actual)
			}
		})
	}
}
