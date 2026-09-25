package topology_transitions

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
)

// TestDetectChain prevents a second transition from running in one suite invocation.
func TestDetectChain(t *testing.T) {
	t.Cleanup(func() { exercisedTransitionName = "" })

	if err := detectChain("a"); err != nil {
		t.Fatalf("first call for a new transition should succeed, got: %v", err)
	}
	if err := detectChain("a"); err != nil {
		t.Fatalf("repeat calls for the same transition should succeed, got: %v", err)
	}
	if err := detectChain("b"); err == nil {
		t.Fatal("expected an error when a different transition tries to run in the same invocation, got nil")
	}
}

// TestControlPlaneTopologyPatch verifies clearing and setting the optional field.
func TestControlPlaneTopologyPatch(t *testing.T) {
	tests := []struct {
		name string
		mode configv1.TopologyMode
		want string
	}{
		{
			name: "empty mode removes the field",
			want: `{"spec":{"controlPlaneTopology":null}}`,
		},
		{
			name: "non-empty mode sets the field",
			mode: configv1.HighlyAvailableTopologyMode,
			want: `{"spec":{"controlPlaneTopology":"HighlyAvailable"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(controlPlaneTopologyPatch(tt.mode)); got != tt.want {
				t.Errorf("controlPlaneTopologyPatch() = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestPlatformTypeOmitsProviderDetails checks that diagnostics expose only the provider type.
func TestPlatformTypeOmitsProviderDetails(t *testing.T) {
	status := &configv1.PlatformStatus{
		Type: configv1.AWSPlatformType,
		AWS: &configv1.AWSPlatformStatus{
			ServiceEndpoints: []configv1.AWSServiceEndpoint{{Name: "ec2", URL: "https://private.example"}},
		},
	}

	if got := platformType(status); got != string(configv1.AWSPlatformType) {
		t.Fatalf("platformType() = %q, want %q", got, configv1.AWSPlatformType)
	}
}

// TestConditionSummaryOmitsMessage checks that diagnostics exclude free-form condition text.
func TestConditionSummaryOmitsMessage(t *testing.T) {
	condition := &operatorv1.OperatorCondition{
		Type:    "Progressing",
		Status:  operatorv1.ConditionFalse,
		Reason:  "AsExpected",
		Message: "private diagnostic text",
	}

	if got, want := conditionSummary(condition), "type=Progressing status=False reason=AsExpected"; got != want {
		t.Fatalf("conditionSummary() = %q, want %q", got, want)
	}
}
