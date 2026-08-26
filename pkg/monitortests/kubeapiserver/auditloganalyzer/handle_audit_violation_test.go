package auditloganalyzer

import (
	"context"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/api/features"
	configfake "github.com/openshift/client-go/config/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func violationCheckerWithRecord() *auditViolations {
	checker := CheckForViolations()
	checker.records = append(checker.records, auditViolationRecord{
		auditId:   "test-audit-id",
		violation: `would violate PodSecurity "restricted:latest"`,
		resource:  "pods",
		namespace: "e2e-test-default-abcde",
		username:  "system:serviceaccount:openshift-operator-lifecycle-manager:olm-operator-serviceaccount",
	})
	return checker
}

func TestCreateJunitsWithPSAEnforcement(t *testing.T) {
	checker := violationCheckerWithRecord()

	junits := checker.CreateJunits(false)

	require.Len(t, junits, 1)
	assert.NotNil(t, junits[0].FailureOutput, "violations must fail the invariant when PSA enforcement is enabled")
	assert.Nil(t, junits[0].SkipMessage)
}

func TestCreateJunitsWithoutPSAEnforcement(t *testing.T) {
	checker := violationCheckerWithRecord()

	junits := checker.CreateJunits(true)

	require.Len(t, junits, 1)
	assert.Nil(t, junits[0].FailureOutput, "violations are expected when PSA enforcement is disabled and must not fail the invariant")
	require.NotNil(t, junits[0].SkipMessage)
	assert.Contains(t, junits[0].SkipMessage.Message, "OpenShiftPodSecurityAdmission is disabled")
}

func TestCreateJunitsWithoutViolations(t *testing.T) {
	checker := CheckForViolations()

	junits := checker.CreateJunits(false)

	require.Len(t, junits, 1)
	assert.Nil(t, junits[0].FailureOutput)
	assert.Nil(t, junits[0].SkipMessage)
}

func TestIsPodSecurityEnforcementDisabled(t *testing.T) {
	clusterVersion := &configv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Status: configv1.ClusterVersionStatus{
			Desired: configv1.Release{Version: "5.0.0"},
		},
	}

	testCases := []struct {
		name        string
		featureGate *configv1.FeatureGate
		expected    bool
	}{
		{
			name: "gate disabled for current version",
			featureGate: featureGateWithStatus("5.0.0", nil, []configv1.FeatureGateName{
				features.FeatureGateOpenShiftPodSecurityAdmission,
			}),
			expected: true,
		},
		{
			name: "gate enabled for current version",
			featureGate: featureGateWithStatus("5.0.0", []configv1.FeatureGateName{
				features.FeatureGateOpenShiftPodSecurityAdmission,
			}, nil),
			expected: false,
		},
		{
			name: "gate disabled only for another version",
			featureGate: featureGateWithStatus("4.20.0", nil, []configv1.FeatureGateName{
				features.FeatureGateOpenShiftPodSecurityAdmission,
			}),
			expected: false,
		},
		{
			name:        "no featuregate resource keeps the invariant enforcing",
			featureGate: nil,
			expected:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configClient := configfake.NewSimpleClientset()
			if tc.featureGate != nil {
				configClient = configfake.NewSimpleClientset(tc.featureGate)
			}

			actual := isPodSecurityEnforcementDisabled(context.Background(), configClient, clusterVersion)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func featureGateWithStatus(version string, enabled, disabled []configv1.FeatureGateName) *configv1.FeatureGate {
	featureGate := &configv1.FeatureGate{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Status: configv1.FeatureGateStatus{
			FeatureGates: []configv1.FeatureGateDetails{
				{Version: version},
			},
		},
	}
	for _, name := range enabled {
		featureGate.Status.FeatureGates[0].Enabled = append(featureGate.Status.FeatureGates[0].Enabled, configv1.FeatureGateAttributes{Name: name})
	}
	for _, name := range disabled {
		featureGate.Status.FeatureGates[0].Disabled = append(featureGate.Status.FeatureGates[0].Disabled, configv1.FeatureGateAttributes{Name: name})
	}
	return featureGate
}
