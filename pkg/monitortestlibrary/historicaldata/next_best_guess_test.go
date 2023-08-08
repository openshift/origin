package historicaldata

import "testing"

func TestCurrentReleaseFromMap(t *testing.T) {
	// Test case: Empty input map
	releasesInQueryResults := make(map[string]bool)
	result := CurrentReleaseFromMap(releasesInQueryResults)
	if result != "" {
		t.Errorf("Expected empty string, but got %s", result)
	}

	// Test case: Non-empty input map
	releasesInQueryResults = map[string]bool{
		"4.10.0": true,
		"4.11.1": false,
		"4.14.0": true,
		"4.12.4": true,
	}
	result = CurrentReleaseFromMap(releasesInQueryResults)
	if result != "4.14.0" {
		t.Errorf("Expected '4.14.0', but got %s", result)
	}
}

func TestCompareReleaseString(t *testing.T) {
	// Test case: Same major version, different minor version (4.11 > 4.10)
	result := compareReleaseString("4.11.0", "4.10.0")
	if !result {
		t.Errorf("Expected true, but got false")
	}

	// Test case: Same major and minor versions (4.12 == 4.12)
	result = compareReleaseString("4.12.0", "4.12.0")
	if result {
		t.Errorf("Expected false, but got true")
	}

	// Test case: Different major versions (5.0 > 4.11)
	result = compareReleaseString("5.0.0", "4.11.0")
	if !result {
		t.Errorf("Expected true, but got false")
	}
}
