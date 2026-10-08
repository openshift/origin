package legacynetworkmonitortests

import (
	"strings"
	"testing"
)

func TestROSANetworkMonitorTestsAreSkipped(t *testing.T) {
	sandboxTests := testPodSandboxCreation(nil, nil, true)
	if len(sandboxTests) == 0 {
		t.Fatal("expected pod sandbox monitor tests")
	}
	foundOther := false
	for _, test := range sandboxTests {
		if test.SkipMessage == nil {
			t.Errorf("pod sandbox test %q was not skipped", test.Name)
		}
		if strings.HasSuffix(test.Name, " by other") {
			foundOther = true
		}
	}
	if !foundOther {
		t.Error("pod sandbox catch-all test was not returned")
	}

	ovsTests := testNoOVSVswitchdUnreasonablyLongPollIntervals(nil, true)
	if len(ovsTests) != 1 {
		t.Fatalf("got %d ovs-vswitchd monitor tests, want 1", len(ovsTests))
	}
	if ovsTests[0].SkipMessage == nil {
		t.Errorf("ovs-vswitchd monitor test %q was not skipped", ovsTests[0].Name)
	}
}
