package ginkgo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSummarizeTests(t *testing.T) {
	tests := []struct {
		name           string
		input          []*testCase
		wantPass       int
		wantFail       int
		wantSkip       int
		wantFlake      int
		wantFlakeNames []string
		failedAttempts int
	}{
		{
			name: "success",
			input: []*testCase{
				{name: "test1", success: true},
				{name: "test2", success: true},
			},
			wantPass:  2,
			wantFail:  0,
			wantSkip:  0,
			wantFlake: 0,
		},
		{
			name: "failure",
			input: []*testCase{
				{name: "test1", failed: true},
			},
			wantPass:       0,
			wantFail:       1,
			failedAttempts: 1,
			wantSkip:       0,
			wantFlake:      0,
		},
		{
			name: "flake",
			input: []*testCase{
				{name: "test1", failed: true},
				{name: "test1", success: true},
			},
			wantPass:       0,
			wantFail:       0,
			wantSkip:       0,
			wantFlake:      1,
			wantFlakeNames: []string{"test1"},
		},
		{
			name: "retried failure",
			input: []*testCase{
				{name: "test1", failed: true},
				{name: "test1", failed: true},
			},
			wantPass:       0,
			wantFail:       2, // Both failed test cases should be counted
			wantSkip:       0,
			wantFlake:      0,
			failedAttempts: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := summarizeTests(tc.input)

			assert.Equal(t, tc.wantPass, len(result.passed), "pass count")
			assert.Equal(t, tc.wantFail, len(result.failed), "fail count")
			assert.Equal(t, tc.wantSkip, len(result.skipped), "skip count")
			assert.Equal(t, tc.wantFlake, len(result.flaked), "flake count")
			assert.Equal(t, tc.failedAttempts, len(result.failed), "failing len")

			// Extract flake names from the flaked groups
			var flakeNames []string
			for _, flakedGroup := range result.flaked {
				if len(flakedGroup) > 0 {
					flakeNames = append(flakeNames, flakedGroup[0].name)
				}
			}
			assert.Equal(t, len(tc.wantFlakeNames), len(flakeNames), "flake name count")

			for _, expectedName := range tc.wantFlakeNames {
				found := false
				for _, actualName := range flakeNames {
					if actualName == expectedName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected flake test: %s", expectedName)
				}
			}
		})
	}
}
