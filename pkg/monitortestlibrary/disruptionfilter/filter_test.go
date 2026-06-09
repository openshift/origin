package disruptionfilter

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func TestFilterOutKnownDisruptiveTestIntervals_KMSEncryptionOnSNO(t *testing.T) {
	now := time.Now()
	kmsTest := monitorapi.Interval{
		Condition: monitorapi.Condition{
			Level: monitorapi.Info,
			Locator: monitorapi.Locator{
				Type: monitorapi.LocatorTypeE2ETest,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorE2ETestKey: "[sig-api-machinery][OCPFeatureGate:KMSEncryption] test encryption",
				},
			},
		},
		Source: monitorapi.SourceE2ETest,
		From:   now,
		To:     now.Add(30 * time.Minute),
	}
	disruptionDuringKMS := monitorapi.Interval{
		Condition: monitorapi.Condition{
			Level: monitorapi.Error,
			Locator: monitorapi.Locator{
				Type: monitorapi.LocatorTypeDisruption,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorBackendDisruptionNameKey: "ingress-to-oauth-server-new-connections",
				},
			},
			Message: monitorapi.Message{
				Reason: monitorapi.DisruptionBeganEventReason,
			},
		},
		Source: monitorapi.SourceDisruption,
		From:   now.Add(5 * time.Minute),
		To:     now.Add(6 * time.Minute),
	}

	filtered := FilterOutKnownDisruptiveTestIntervals(monitorapi.Intervals{kmsTest, disruptionDuringKMS}, "single")
	if len(filtered) != 1 {
		t.Fatalf("expected disruption during KMS test to be filtered on SNO, got %d intervals", len(filtered))
	}
}

func TestFilterOutKnownDisruptiveTestIntervals_NotFilteredOnSNOWithoutKMSTests(t *testing.T) {
	now := time.Now()
	disruption := monitorapi.Interval{
		Condition: monitorapi.Condition{
			Level: monitorapi.Error,
			Locator: monitorapi.Locator{
				Type: monitorapi.LocatorTypeDisruption,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorBackendDisruptionNameKey: "ingress-to-oauth-server-new-connections",
				},
			},
			Message: monitorapi.Message{
				Reason: monitorapi.DisruptionBeganEventReason,
			},
		},
		Source: monitorapi.SourceDisruption,
		From:   now,
		To:     now.Add(time.Minute),
	}

	filtered := FilterOutKnownDisruptiveTestIntervals(monitorapi.Intervals{disruption}, "single")
	if len(filtered) != 1 {
		t.Fatalf("expected disruption to remain on SNO without KMS tests, got %d intervals", len(filtered))
	}
}

func TestFilterOutKnownDisruptiveTestIntervals_KMSEncryptionNotFilteredOnHA(t *testing.T) {
	now := time.Now()
	kmsTest := monitorapi.Interval{
		Condition: monitorapi.Condition{
			Level: monitorapi.Info,
			Locator: monitorapi.Locator{
				Type: monitorapi.LocatorTypeE2ETest,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorE2ETestKey: "[sig-api-machinery][OCPFeatureGate:KMSEncryption] test encryption",
				},
			},
		},
		Source: monitorapi.SourceE2ETest,
		From:   now,
		To:     now.Add(30 * time.Minute),
	}
	disruptionDuringKMS := monitorapi.Interval{
		Condition: monitorapi.Condition{
			Level: monitorapi.Error,
			Locator: monitorapi.Locator{
				Type: monitorapi.LocatorTypeDisruption,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorBackendDisruptionNameKey: "cache-kube-api-reused-connections",
				},
			},
			Message: monitorapi.Message{
				Reason: monitorapi.DisruptionBeganEventReason,
			},
		},
		Source: monitorapi.SourceDisruption,
		From:   now.Add(5 * time.Minute),
		To:     now.Add(6 * time.Minute),
	}

	filtered := FilterOutKnownDisruptiveTestIntervals(monitorapi.Intervals{kmsTest, disruptionDuringKMS}, "ha")
	if len(filtered) != 2 {
		t.Fatalf("expected KMS disruption to remain on HA, got %d intervals", len(filtered))
	}
}
