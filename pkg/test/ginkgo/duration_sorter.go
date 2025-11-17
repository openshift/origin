package ginkgo

import (
	_ "embed"
	"encoding/json"
	"sort"

	"github.com/sirupsen/logrus"
)

//go:embed testDurations.json
var testDurationsData []byte

// TestDurationData represents the duration information for a single test
type TestDurationData struct {
	AverageDuration int `json:"average_duration"`
}

// testDurations holds the parsed duration data
var testDurations map[string]TestDurationData

// init parses the embedded duration data
func init() {
	if err := json.Unmarshal(testDurationsData, &testDurations); err != nil {
		logrus.Warnf("Failed to parse embedded test durations: %v", err)
		testDurations = make(map[string]TestDurationData)
	}
	logrus.Infof("Loaded duration data for %d tests", len(testDurations))
}

// SortTestsByDuration sorts tests by duration (longest to shortest).
// Tests without duration data are placed at the end in their original order.
func SortTestsByDuration(tests []*testCase) {
	sort.SliceStable(tests, func(i, j int) bool {
		iDuration, iExists := testDurations[tests[i].name]
		jDuration, jExists := testDurations[tests[j].name]

		// If neither test has duration data, maintain original order
		if !iExists && !jExists {
			return false
		}

		// If only one test has duration data, put it first
		if iExists && !jExists {
			return true
		}
		if !iExists && jExists {
			return false
		}

		// Both tests have duration data - sort by duration (longest first)
		return iDuration.AverageDuration > jDuration.AverageDuration
	})

	// Log some statistics about the sorting
	withDuration := 0
	for _, test := range tests {
		if _, exists := testDurations[test.name]; exists {
			withDuration++
		}
	}
	logrus.Infof("Sorted %d tests by duration (%d with duration data, %d without)",
		len(tests), withDuration, len(tests)-withDuration)
}
