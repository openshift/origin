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
