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
