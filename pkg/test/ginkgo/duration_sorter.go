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
	P50Duration     int `json:"p50_duration"`
	P90Duration     int `json:"p90_duration"`
	P95Duration     int `json:"p95_duration"`
	P99Duration     int `json:"p99_duration"`
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

		// Both tests have duration data - sort by p95 duration (longest first)
		return iDuration.P95Duration > jDuration.P95Duration
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

// ExtractLongRunningBucket extracts tests for a "long running" bucket using bin-packing.
// It distributes tests across N parallel bins (where N = parallelism) to maximize utilization.
// Tests are assigned to bins using a greedy algorithm that balances total duration across bins.
// Returns the tests selected for the long-running bucket and the remaining tests.
func ExtractLongRunningBucket(tests []*testCase, parallelism int) ([]*testCase, []*testCase) {
	if parallelism <= 0 || len(tests) == 0 {
		return nil, tests
	}

	// Track duration for each parallel bin/worker
	type bin struct {
		totalDuration int
		tests         []*testCase
	}
	bins := make([]bin, parallelism)

	// Separate tests with and without duration data
	var testsWithDuration []*testCase
	var testsWithoutDuration []*testCase

	for _, test := range tests {
		if _, exists := testDurations[test.name]; exists {
			testsWithDuration = append(testsWithDuration, test)
		} else {
			testsWithoutDuration = append(testsWithoutDuration, test)
		}
	}

	if len(testsWithDuration) == 0 {
		logrus.Info("No tests with duration data available for long-running bucket")
		return nil, tests
	}

	// Get the longest test p95 duration to use as our target
	longestDuration := testDurations[testsWithDuration[0].name].P95Duration

	// Bin-packing: assign each test to the bin with smallest current total duration
	for _, test := range testsWithDuration {
		duration := testDurations[test.name].P95Duration

		// Find bin with minimum total duration
		minBinIdx := 0
		minDuration := bins[0].totalDuration
		for i := 1; i < len(bins); i++ {
			if bins[i].totalDuration < minDuration {
				minDuration = bins[i].totalDuration
				minBinIdx = i
			}
		}

		// Assign test to this bin
		bins[minBinIdx].tests = append(bins[minBinIdx].tests, test)
		bins[minBinIdx].totalDuration += duration

		// Stop when the minimum bin duration reaches the longest test duration
		// This ensures we pack enough tests to keep all workers busy for at least
		// as long as the longest single test
		if bins[minBinIdx].totalDuration >= longestDuration {
			// Check if all bins have at least one test
			allBinsHaveTests := true
			for i := range bins {
				if len(bins[i].tests) == 0 {
					allBinsHaveTests = false
					break
				}
			}
			if allBinsHaveTests {
				break
			}
		}
	}

	// Collect all tests assigned to bins
	var longRunningTests []*testCase
	testSet := make(map[*testCase]bool)

	totalDuration := 0
	for i, bin := range bins {
		logrus.Infof("Long-running bin %d: %d tests, total duration: %d seconds",
			i, len(bin.tests), bin.totalDuration)
		totalDuration += bin.totalDuration
		for _, test := range bin.tests {
			if !testSet[test] {
				testSet[test] = true
				longRunningTests = append(longRunningTests, test)
			}
		}
	}

	avgDuration := 0
	if len(bins) > 0 {
		avgDuration = totalDuration / len(bins)
	}
	logrus.Infof("Long-running bucket: %d tests across %d bins, avg bin duration: %d seconds",
		len(longRunningTests), len(bins), avgDuration)

	// Create list of remaining tests (those not in long-running bucket)
	var remainingTests []*testCase
	for _, test := range tests {
		if !testSet[test] {
			remainingTests = append(remainingTests, test)
		}
	}

	return longRunningTests, remainingTests
}

// WriteBucketDebugFile writes a debug file containing the ordered list of tests
// in a bucket along with their durations from testDurations.json
func WriteBucketDebugFile(bucketName string, tests []*testCase, junitDir string) {
	if junitDir == "" {
		// Skip writing debug file if no junit directory is specified
		return
	}

	// Ensure the debug directory exists
	if err := os.MkdirAll(junitDir, 0755); err != nil {
		logrus.Warnf("Failed to create debug directory %s: %v", junitDir, err)
		return
	}

	filename := filepath.Join(junitDir, fmt.Sprintf("bucket_%s.txt", bucketName))
	f, err := os.Create(filename)
	if err != nil {
		logrus.Warnf("Failed to create debug file %s: %v", filename, err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "Bucket: %s\n", bucketName)
	fmt.Fprintf(f, "Total tests: %d\n\n", len(tests))
	fmt.Fprintf(f, "%-10s %-10s %-10s %-10s %-10s %-10s %s\n", "Duration", "P50", "P90", "P95", "P99", "Avg", "Test Name")
	fmt.Fprintf(f, "%-10s %-10s %-10s %-10s %-10s %-10s %s\n", "--------", "---", "---", "---", "---", "---", "---------")

	for _, test := range tests {
		if duration, exists := testDurations[test.name]; exists {
			fmt.Fprintf(f, "%-10d %-10d %-10d %-10d %-10d %-10d %s\n",
				duration.P95Duration, duration.P50Duration, duration.P90Duration,
				duration.P95Duration, duration.P99Duration, duration.AverageDuration, test.name)
		} else {
			fmt.Fprintf(f, "%-10s %-10s %-10s %-10s %-10s %-10s %s\n", "N/A", "N/A", "N/A", "N/A", "N/A", "N/A", test.name)
		}
	}

	logrus.Infof("Wrote debug file for bucket %s to %s", bucketName, filename)
}
