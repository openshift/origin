package upgrade

import (
	"fmt"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

// TestReleaseAcceptedForTarget verifies that releaseAcceptedForTarget only returns the CVO
// ReleaseAccepted condition when it refers to the requested update (matched by image, or by
// version for version-only requests), ignoring stale conditions left over from other releases.
func TestReleaseAcceptedForTarget(t *testing.T) {
	const (
		targetVersion = "4.99.0"
		targetImage   = "quay.io/openshift-release-dev/ocp-release@sha256:aaaa"
		otherVersion  = "4.98.0"
		otherImage    = "quay.io/openshift-release-dev/ocp-release@sha256:bbbb"
	)

	retrieving := func(version, image string) configv1.ClusterOperatorStatusCondition {
		return configv1.ClusterOperatorStatusCondition{
			Type:    releaseAcceptedConditionType,
			Status:  configv1.ConditionUnknown,
			Reason:  "RetrievePayload",
			Message: fmt.Sprintf("Retrieving and verifying payload version=%q image=%q", version, image),
		}
	}
	failed := func(version, image string) configv1.ClusterOperatorStatusCondition {
		return configv1.ClusterOperatorStatusCondition{
			Type:    releaseAcceptedConditionType,
			Status:  configv1.ConditionFalse,
			Reason:  "RetrievePayload",
			Message: fmt.Sprintf("Retrieving payload failed version=%q image=%q failure=boom", version, image),
		}
	}

	cvWith := func(conds ...configv1.ClusterOperatorStatusCondition) *configv1.ClusterVersion {
		return &configv1.ClusterVersion{Status: configv1.ClusterVersionStatus{Conditions: conds}}
	}

	tests := []struct {
		name       string
		cv         *configv1.ClusterVersion
		desired    configv1.Update
		wantMatch  bool
		wantStatus configv1.ConditionStatus
	}{
		{
			name:      "no ReleaseAccepted condition yet",
			cv:        cvWith(),
			desired:   configv1.Update{Version: targetVersion, Image: targetImage},
			wantMatch: false,
		},
		{
			name:       "image-matched retrieval in progress",
			cv:         cvWith(retrieving(targetVersion, targetImage)),
			desired:    configv1.Update{Version: targetVersion, Image: targetImage},
			wantMatch:  true,
			wantStatus: configv1.ConditionUnknown,
		},
		{
			name:       "image-matched retrieval failure",
			cv:         cvWith(failed(targetVersion, targetImage)),
			desired:    configv1.Update{Version: targetVersion, Image: targetImage},
			wantMatch:  true,
			wantStatus: configv1.ConditionFalse,
		},
		{
			name:      "stale condition for a different image is ignored",
			cv:        cvWith(retrieving(otherVersion, otherImage)),
			desired:   configv1.Update{Version: targetVersion, Image: targetImage},
			wantMatch: false,
		},
		{
			// An image-based request must not fall back to version matching: a stale
			// condition whose version happens to match but whose image differs is not
			// our target.
			name:      "image request ignores matching version with different image",
			cv:        cvWith(retrieving(targetVersion, otherImage)),
			desired:   configv1.Update{Version: targetVersion, Image: targetImage},
			wantMatch: false,
		},
		{
			name:       "version-only request matches on version",
			cv:         cvWith(retrieving(targetVersion, otherImage)),
			desired:    configv1.Update{Version: targetVersion},
			wantMatch:  true,
			wantStatus: configv1.ConditionUnknown,
		},
		{
			name:      "version-only request ignores non-matching version",
			cv:        cvWith(retrieving(otherVersion, otherImage)),
			desired:   configv1.Update{Version: targetVersion},
			wantMatch: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := releaseAcceptedForTarget(test.cv, test.desired)
			if test.wantMatch != (got != nil) {
				t.Fatalf("releaseAcceptedForTarget() match = %v, want %v (got %+v)", got != nil, test.wantMatch, got)
			}
			if got != nil && got.Status != test.wantStatus {
				t.Fatalf("releaseAcceptedForTarget() status = %q, want %q", got.Status, test.wantStatus)
			}
		})
	}
}

// TestIsTerminalReleaseAcceptedFailure verifies that only ReleaseAccepted=False conditions whose
// reason the CVO will not recover from by retrying are treated as terminal; a RetrievePayload
// failure (a retried, possibly slow/auth pull) and any non-False condition are not terminal.
func TestIsTerminalReleaseAcceptedFailure(t *testing.T) {
	cond := func(status configv1.ConditionStatus, reason string) *configv1.ClusterOperatorStatusCondition {
		return &configv1.ClusterOperatorStatusCondition{Type: releaseAcceptedConditionType, Status: status, Reason: reason}
	}

	tests := []struct {
		name      string
		condition *configv1.ClusterOperatorStatusCondition
		want      bool
	}{
		{name: "nil condition", condition: nil, want: false},
		{name: "retrieval failure is retriable, not terminal", condition: cond(configv1.ConditionFalse, "RetrievePayload"), want: false},
		{name: "retrieving in progress is not terminal", condition: cond(configv1.ConditionUnknown, "RetrievePayload"), want: false},
		{name: "payload loaded is not terminal", condition: cond(configv1.ConditionTrue, "PayloadLoaded"), want: false},
		{name: "load payload failure is terminal", condition: cond(configv1.ConditionFalse, "LoadPayload"), want: true},
		{name: "version verification failure is terminal", condition: cond(configv1.ConditionFalse, "VerifyPayloadVersion"), want: true},
		{name: "precondition failure is terminal", condition: cond(configv1.ConditionFalse, "PreconditionChecks"), want: true},
		{name: "terminal reason but status not False is not terminal", condition: cond(configv1.ConditionUnknown, "PreconditionChecks"), want: false},
		{name: "unknown reason is not terminal", condition: cond(configv1.ConditionFalse, "SomethingElse"), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isTerminalReleaseAcceptedFailure(test.condition); got != test.want {
				t.Fatalf("isTerminalReleaseAcceptedFailure() = %v, want %v", got, test.want)
			}
		})
	}
}

// TestAckProgress verifies the acknowledgement decision: success once observedGeneration catches up,
// fail-fast only on a terminal payload rejection, and "keep waiting" while a retrieval is slow or
// retrying (so a slow/auth pull is bounded by the caller's hard cap rather than failed immediately).
func TestAckProgress(t *testing.T) {
	cond := func(status configv1.ConditionStatus, reason string) *configv1.ClusterOperatorStatusCondition {
		return &configv1.ClusterOperatorStatusCondition{Type: releaseAcceptedConditionType, Status: status, Reason: reason}
	}

	tests := []struct {
		name            string
		observed        int64
		requested       int64
		releaseAccepted *configv1.ClusterOperatorStatusCondition
		wantDone        bool
		wantErr         bool
	}{
		{name: "observedGeneration caught up succeeds", observed: 3, requested: 3, releaseAccepted: nil, wantDone: true, wantErr: false},
		{name: "observedGeneration ahead succeeds", observed: 4, requested: 3, releaseAccepted: nil, wantDone: true, wantErr: false},
		{name: "no condition yet keeps waiting", observed: 2, requested: 3, releaseAccepted: nil, wantDone: false, wantErr: false},
		{name: "retrieval in progress keeps waiting", observed: 2, requested: 3, releaseAccepted: cond(configv1.ConditionUnknown, "RetrievePayload"), wantDone: false, wantErr: false},
		{name: "retrieval failure keeps waiting (retried)", observed: 2, requested: 3, releaseAccepted: cond(configv1.ConditionFalse, "RetrievePayload"), wantDone: false, wantErr: false},
		{name: "terminal rejection fails fast", observed: 2, requested: 3, releaseAccepted: cond(configv1.ConditionFalse, "PreconditionChecks"), wantDone: false, wantErr: true},
		{name: "acceptance wins even if a stale terminal condition is present", observed: 3, requested: 3, releaseAccepted: cond(configv1.ConditionFalse, "PreconditionChecks"), wantDone: true, wantErr: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			done, err := ackProgress(test.observed, test.requested, test.releaseAccepted)
			if done != test.wantDone {
				t.Fatalf("ackProgress() done = %v, want %v", done, test.wantDone)
			}
			if (err != nil) != test.wantErr {
				t.Fatalf("ackProgress() err = %v, want error: %v", err, test.wantErr)
			}
		})
	}
}
