package topology_transitions

import (
	"context"
	"errors"
	"strings"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
)

func TestUncordonNodesAttemptsAllNodes(t *testing.T) {
	tests := []struct {
		name     string
		failNode string
	}{
		{name: "all succeed"},
		{name: "first node fails", failNode: "node-a"},
		{name: "last node fails", failNode: "node-c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var attempts []string
			err := uncordonNodes(context.Background(), []string{"node-a", "node-b", "node-c"}, func(_ context.Context, name string) error {
				attempts = append(attempts, name)
				if name == tt.failNode {
					return errors.New("API error")
				}
				return nil
			})
			if got := strings.Join(attempts, ","); got != "node-a,node-b,node-c" {
				t.Errorf("attempted %s, want all nodes", got)
			}
			if tt.failNode == "" && err != nil || tt.failNode != "" && (err == nil || !strings.Contains(err.Error(), tt.failNode)) {
				t.Errorf("uncordonNodes() error = %v, want failure for %q", err, tt.failNode)
			}
		})
	}
}

func TestUncordonNodesReportsEveryFailure(t *testing.T) {
	err := uncordonNodes(context.Background(), []string{"node-a", "node-b", "node-c"}, func(_ context.Context, name string) error {
		if name == "node-b" {
			return nil
		}
		return errors.New("API error")
	})
	if err == nil || !strings.Contains(err.Error(), "node-a") || !strings.Contains(err.Error(), "node-c") {
		t.Fatalf("uncordonNodes() error = %v, want both failed node names", err)
	}
}

func TestRestoreRejectedTransition(t *testing.T) {
	tests := []struct {
		name             string
		requestAttempted bool
		resetErr         error
		idleErr          error
		uncordonErr      error
		wantCalls        string
		wantErr          string
	}{
		{name: "no request uncordons without waiting", wantCalls: "uncordon:node-a,uncordon:node-b"},
		{name: "requested transition resets before uncordoning", requestAttempted: true, wantCalls: "reset,idle,uncordon:node-a,uncordon:node-b"},
		{name: "failed reset leaves nodes cordoned", requestAttempted: true, resetErr: errors.New("patch failed"), wantCalls: "reset", wantErr: "patch failed"},
		{name: "controller not idle leaves nodes cordoned", requestAttempted: true, idleErr: errors.New("timed out"), wantCalls: "reset,idle", wantErr: "timed out"},
		{name: "failed uncordon still tries remaining nodes", requestAttempted: true, uncordonErr: errors.New("API error"), wantCalls: "reset,idle,uncordon:node-a,uncordon:node-b", wantErr: "node-a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			err := restoreRejectedTransition(context.Background(), tt.requestAttempted,
				func(context.Context) error { calls = append(calls, "reset"); return tt.resetErr },
				func(context.Context) error { calls = append(calls, "idle"); return tt.idleErr },
				[]string{"node-a", "node-b"},
				func(_ context.Context, name string) error {
					calls = append(calls, "uncordon:"+name)
					if name == "node-a" {
						return tt.uncordonErr
					}
					return nil
				},
			)
			if got := strings.Join(calls, ","); got != tt.wantCalls {
				t.Errorf("calls = %q, want %q", got, tt.wantCalls)
			}
			if tt.wantErr == "" && err != nil || tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("restoreRejectedTransition() error = %v, want text %q", err, tt.wantErr)
			}
		})
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
