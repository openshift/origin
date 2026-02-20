package ginkgo

import (
	"testing"
)

func TestLongTestsLoaded(t *testing.T) {
	if len(longTestsData) == 0 {
		t.Fatal("longTestsData should not be empty after init()")
	}

	expectedGroups := 13
	if len(longTestsData) != expectedGroups {
		t.Errorf("Expected %d groups, got %d", expectedGroups, len(longTestsData))
	}

	// Verify total test count
	totalTests := 0
	for _, group := range longTestsData {
		totalTests += len(group.Tests)
	}

	if totalTests == 0 {
		t.Error("Expected some tests to be loaded")
	}

	t.Logf("Loaded %d groups with %d total tests", len(longTestsData), totalTests)
}

func TestGetTestDuration(t *testing.T) {
	// Test with a known long-running test
	testName := "[sig-apps] poddisruptionbudgets with unhealthyPodEvictionPolicy should evict according to the IfHealthyBudget policy [Suite:openshift/conformance/parallel]"
	duration := GetTestDuration(testName)

	if duration == 0 {
		t.Errorf("Expected non-zero duration for known test %s", testName)
	}

	// Should be 664 seconds based on our data
	expectedDuration := 664
	if duration != expectedDuration {
		t.Errorf("Expected duration %d for test, got %d", expectedDuration, duration)
	}

	// Test with unknown test
	unknownTest := "This test does not exist"
	unknownDuration := GetTestDuration(unknownTest)
	if unknownDuration != 0 {
		t.Errorf("Expected 0 duration for unknown test, got %d", unknownDuration)
	}
}

func TestGetTestGroup(t *testing.T) {
	// Test with a known test
	testName := "[sig-apps] poddisruptionbudgets with unhealthyPodEvictionPolicy should evict according to the IfHealthyBudget policy [Suite:openshift/conformance/parallel]"
	group := GetTestGroup(testName)

	expectedGroup := "sig-apps"
	if group != expectedGroup {
		t.Errorf("Expected group %s for test, got %s", expectedGroup, group)
	}

	// Test with storage test
	storageTest := "[sig-storage] Managed cluster should have no crashlooping recycler pods over four minutes [Suite:openshift/conformance/parallel]"
	storageGroup := GetTestGroup(storageTest)
	if storageGroup != "sig-storage" {
		t.Errorf("Expected group sig-storage, got %s", storageGroup)
	}

	// Test with unknown test
	unknownTest := "This test does not exist"
	unknownGroup := GetTestGroup(unknownTest)
	if unknownGroup != "" {
		t.Errorf("Expected empty group for unknown test, got %s", unknownGroup)
	}
}

func TestLongTestsGroupCoverage(t *testing.T) {
	// Verify we have coverage across multiple SIG groups
	groupCounts := make(map[string]int)
	for _, group := range longTestsData {
		groupCounts[group.GroupID] = len(group.Tests)
	}

	expectedGroups := []string{
		"sig-storage",
		"sig-network",
		"sig-node",
		"sig-apps",
		"sig-builds",
	}

	for _, expectedGroup := range expectedGroups {
		count, exists := groupCounts[expectedGroup]
		if !exists {
			t.Errorf("Expected group %s to exist in long_tests.json", expectedGroup)
		}
		if count == 0 {
			t.Errorf("Expected group %s to have tests, got 0", expectedGroup)
		}
		t.Logf("Group %s has %d tests", expectedGroup, count)
	}
}

func TestLongTestsSortedByDuration(t *testing.T) {
	// Verify that tests within each group are sorted by duration (descending)
	for _, group := range longTestsData {
		if len(group.Tests) < 2 {
			continue // Skip groups with only one test
		}

		prevDuration := group.Tests[0].DurationSeconds
		for i := 1; i < len(group.Tests); i++ {
			currentDuration := group.Tests[i].DurationSeconds
			if currentDuration > prevDuration {
				t.Errorf("Group %s: Test at index %d has duration %.0f which is greater than previous test duration %.0f. Tests should be sorted longest first.",
					group.GroupID, i, currentDuration, prevDuration)
			}
			prevDuration = currentDuration
		}

		// Log the first and last test durations
		t.Logf("Group %s: First test duration=%.0fs, Last test duration=%.0fs",
			group.GroupID, group.Tests[0].DurationSeconds, group.Tests[len(group.Tests)-1].DurationSeconds)
	}
}

func TestNeverFailTestsLoaded(t *testing.T) {
	if neverFailTestNames == nil {
		t.Fatal("neverFailTestNames should not be nil after init()")
	}

	if len(neverFailTestNames) == 0 {
		t.Error("Expected some never-fail tests to be loaded")
	}

	t.Logf("Loaded %d never-fail tests", len(neverFailTestNames))
}

func TestIsNeverFailTest(t *testing.T) {
	// Test with a known never-fail test (from the file we checked earlier)
	neverFailTest := "[bz-Cluster Version Operator] Verify presence of admin ack gate blocks upgrade until acknowledged"
	if !IsNeverFailTest(neverFailTest) {
		t.Errorf("Expected test to be marked as never-fail: %s", neverFailTest)
	}

	// Test with a test that's not in the never-fail list
	regularTest := "[sig-apps] poddisruptionbudgets with unhealthyPodEvictionPolicy should evict according to the IfHealthyBudget policy [Suite:openshift/conformance/parallel]"
	if IsNeverFailTest(regularTest) {
		t.Errorf("Expected test NOT to be marked as never-fail: %s", regularTest)
	}

	// Test with unknown test
	unknownTest := "This test does not exist"
	if IsNeverFailTest(unknownTest) {
		t.Errorf("Expected unknown test NOT to be marked as never-fail: %s", unknownTest)
	}
}
