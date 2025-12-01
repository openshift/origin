package ginkgo

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// WriteBucketDebugFile writes a debug file containing the ordered list of tests
// in a bucket along with their durations from testDurations.json
func WriteBucketDebugFile(bucketName string, tests []*testCase) {
	debugDir := os.Getenv("TEST_BUCKET_DEBUG_DIR")
	if debugDir == "" {
		debugDir = "." // Default to current directory
	}

	// Ensure the debug directory exists
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		logrus.Warnf("Failed to create debug directory %s: %v", debugDir, err)
		return
	}

	filename := filepath.Join(debugDir, fmt.Sprintf("bucket_%s.txt", bucketName))
	f, err := os.Create(filename)
	if err != nil {
		logrus.Warnf("Failed to create debug file %s: %v", filename, err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "Bucket: %s\n", bucketName)
	fmt.Fprintf(f, "Total tests: %d\n\n", len(tests))
	fmt.Fprintf(f, "%-10s %s\n", "Duration", "Test Name")
	fmt.Fprintf(f, "%-10s %s\n", "--------", "---------")

	for _, test := range tests {
		if duration, exists := testDurations[test.name]; exists {
			fmt.Fprintf(f, "%-10d %s\n", duration.AverageDuration, test.name)
		} else {
			fmt.Fprintf(f, "%-10s %s\n", "N/A", test.name)
		}
	}

	logrus.Infof("Wrote debug file for bucket %s to %s", bucketName, filename)
}
