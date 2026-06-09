package disruptionfilter

import (
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

const (
	kmsEncryptionFeatureGateTag = "[OCPFeatureGate:KMSEncryption]"
	kmsTestIntervalBuffer       = 5 * time.Minute
)

// FilterOutKnownDisruptiveTestIntervals removes disruption intervals that overlap with
// known-disruptive serial tests like NoExecuteTaintManager, which applies NoExecute taints
// to worker nodes where its test pods land, evicting pods and causing expected unavailability.
//
// On SNO jobs that run OCP KMS encryption tests, disruption overlapping those tests is
// also removed. KMS tests trigger kube-apiserver rollouts that cascade to oauth and
// apiserver backends.
func FilterOutKnownDisruptiveTestIntervals(intervals monitorapi.Intervals, topology string) monitorapi.Intervals {
	knownDisruptiveTests := intervals.Filter(func(i monitorapi.Interval) bool {
		if i.Source != monitorapi.SourceE2ETest {
			return false
		}
		testName := i.Locator.Keys[monitorapi.LocatorE2ETestKey]
		return strings.Contains(testName, "NoExecuteTaintManager")
	})

	if topology == "single" && kmsEncryptionTestsDetected(intervals) {
		knownDisruptiveTests = append(knownDisruptiveTests, bufferedKMSEncryptionTestIntervals(intervals)...)
	}

	if len(knownDisruptiveTests) == 0 {
		return intervals
	}

	return intervals.Filter(func(i monitorapi.Interval) bool {
		for _, disruptiveTest := range knownDisruptiveTests {
			if intervalsOverlap(i, disruptiveTest) && monitorapi.IsErrorEvent(i) {
				return false
			}
		}
		return true
	})
}

func kmsEncryptionTestsDetected(intervals monitorapi.Intervals) bool {
	return len(kmsEncryptionTestIntervals(intervals)) > 0
}

func kmsEncryptionTestIntervals(intervals monitorapi.Intervals) monitorapi.Intervals {
	return intervals.Filter(func(i monitorapi.Interval) bool {
		if i.Source != monitorapi.SourceE2ETest {
			return false
		}
		return strings.Contains(i.Locator.Keys[monitorapi.LocatorE2ETestKey], kmsEncryptionFeatureGateTag)
	})
}

func bufferedKMSEncryptionTestIntervals(intervals monitorapi.Intervals) monitorapi.Intervals {
	kmsTests := kmsEncryptionTestIntervals(intervals)
	for i := range kmsTests {
		kmsTests[i].From = kmsTests[i].From.Add(-kmsTestIntervalBuffer)
		kmsTests[i].To = kmsTests[i].To.Add(kmsTestIntervalBuffer)
	}
	return kmsTests
}

func intervalsOverlap(interval1, interval2 monitorapi.Interval) bool {
	end1 := interval1.To
	if end1.IsZero() {
		end1 = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	}

	end2 := interval2.To
	if end2.IsZero() {
		end2 = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	}

	return (interval1.From.Before(end2)) && (interval2.From.Before(end1))
}
