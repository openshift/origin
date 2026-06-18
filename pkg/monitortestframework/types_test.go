package monitortestframework

import (
	"testing"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

func TestJUnitsToFlakes(t *testing.T) {
	tests := []struct {
		name     string
		input    []*junitapi.JUnitTestCase
		expected []*junitapi.JUnitTestCase
	}{
		{
			name:     "nil input returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty input returns empty",
			input:    []*junitapi.JUnitTestCase{},
			expected: []*junitapi.JUnitTestCase{},
		},
		{
			name: "pass-only test unchanged",
			input: []*junitapi.JUnitTestCase{
				{Name: "test-a"},
			},
			expected: []*junitapi.JUnitTestCase{
				{Name: "test-a"},
			},
		},
		{
			name: "multiple pass-only tests unchanged",
			input: []*junitapi.JUnitTestCase{
				{Name: "test-a"},
				{Name: "test-b"},
			},
			expected: []*junitapi.JUnitTestCase{
				{Name: "test-a"},
				{Name: "test-b"},
			},
		},
		{
			name: "failure-only gets pass appended to become flake",
			input: []*junitapi.JUnitTestCase{
				{
					Name:          "test-a",
					FailureOutput: &junitapi.FailureOutput{Output: "something broke"},
				},
			},
			expected: []*junitapi.JUnitTestCase{
				{
					Name:          "test-a",
					FailureOutput: &junitapi.FailureOutput{Output: "something broke"},
				},
				{Name: "test-a"},
			},
		},
		{
			name: "existing flake (fail+pass) unchanged",
			input: []*junitapi.JUnitTestCase{
				{
					Name:          "test-a",
					FailureOutput: &junitapi.FailureOutput{Output: "something broke"},
				},
				{Name: "test-a"},
			},
			expected: []*junitapi.JUnitTestCase{
				{
					Name:          "test-a",
					FailureOutput: &junitapi.FailureOutput{Output: "something broke"},
				},
				{Name: "test-a"},
			},
		},
		{
			name: "mix of pass, fail, and already-flake",
			input: []*junitapi.JUnitTestCase{
				// test-a: pass only
				{Name: "test-a"},
				// test-b: fail only, should get pass appended
				{
					Name:          "test-b",
					FailureOutput: &junitapi.FailureOutput{Output: "b broke"},
				},
				// test-c: already a flake (fail + pass)
				{
					Name:          "test-c",
					FailureOutput: &junitapi.FailureOutput{Output: "c broke"},
				},
				{Name: "test-c"},
			},
			expected: []*junitapi.JUnitTestCase{
				{Name: "test-a"},
				{
					Name:          "test-b",
					FailureOutput: &junitapi.FailureOutput{Output: "b broke"},
				},
				{
					Name:          "test-c",
					FailureOutput: &junitapi.FailureOutput{Output: "c broke"},
				},
				{Name: "test-c"},
				// appended pass for test-b
				{Name: "test-b"},
			},
		},
		{
			name: "skip-only test unchanged",
			input: []*junitapi.JUnitTestCase{
				{
					Name:        "test-a",
					SkipMessage: &junitapi.SkipMessage{Message: "not supported"},
				},
			},
			expected: []*junitapi.JUnitTestCase{
				{
					Name:        "test-a",
					SkipMessage: &junitapi.SkipMessage{Message: "not supported"},
				},
			},
		},
		{
			name: "skip does not count as pass for a failure",
			input: []*junitapi.JUnitTestCase{
				{
					Name:          "test-a",
					FailureOutput: &junitapi.FailureOutput{Output: "broke"},
				},
				{
					Name:        "test-a",
					SkipMessage: &junitapi.SkipMessage{Message: "not supported"},
				},
			},
			expected: []*junitapi.JUnitTestCase{
				{
					Name:          "test-a",
					FailureOutput: &junitapi.FailureOutput{Output: "broke"},
				},
				{
					Name:        "test-a",
					SkipMessage: &junitapi.SkipMessage{Message: "not supported"},
				},
				{Name: "test-a"},
			},
		},
		{
			name: "multiple failures for same test get single pass",
			input: []*junitapi.JUnitTestCase{
				{
					Name:          "test-a",
					FailureOutput: &junitapi.FailureOutput{Output: "first failure"},
				},
				{
					Name:          "test-a",
					FailureOutput: &junitapi.FailureOutput{Output: "second failure"},
				},
			},
			expected: []*junitapi.JUnitTestCase{
				{
					Name:          "test-a",
					FailureOutput: &junitapi.FailureOutput{Output: "first failure"},
				},
				{
					Name:          "test-a",
					FailureOutput: &junitapi.FailureOutput{Output: "second failure"},
				},
				{Name: "test-a"},
			},
		},
		{
			name: "nil entries in slice are tolerated",
			input: []*junitapi.JUnitTestCase{
				nil,
				{
					Name:          "test-a",
					FailureOutput: &junitapi.FailureOutput{Output: "broke"},
				},
				nil,
			},
			expected: []*junitapi.JUnitTestCase{
				nil,
				{
					Name:          "test-a",
					FailureOutput: &junitapi.FailureOutput{Output: "broke"},
				},
				nil,
				{Name: "test-a"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JUnitsToFlakes(tt.input)

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d junits, got %d\nexpected: %s\ngot:      %s",
					len(tt.expected), len(result), formatJUnits(tt.expected), formatJUnits(result))
			}

			for i := range result {
				if !junitEqual(result[i], tt.expected[i]) {
					t.Errorf("junit[%d] mismatch\nexpected: %s\ngot:      %s",
						i, formatJUnit(tt.expected[i]), formatJUnit(result[i]))
				}
			}
		})
	}
}

func TestJUnitsToFlakes_DoesNotMutateOriginalFailures(t *testing.T) {
	original := &junitapi.JUnitTestCase{
		Name:          "test-a",
		FailureOutput: &junitapi.FailureOutput{Output: "broke"},
	}
	input := []*junitapi.JUnitTestCase{original}
	result := JUnitsToFlakes(input)

	if len(result) != 2 {
		t.Fatalf("expected 2 junits, got %d", len(result))
	}
	// The original failure entry should still be at index 0, unmodified.
	if result[0].FailureOutput == nil || result[0].FailureOutput.Output != "broke" {
		t.Error("original failure entry was mutated")
	}
	// The appended pass entry should be a distinct object.
	if result[1].FailureOutput != nil {
		t.Error("appended entry should be a pass (no FailureOutput)")
	}
	if result[1].Name != "test-a" {
		t.Errorf("appended entry has wrong name: %q", result[1].Name)
	}
}

func junitEqual(a, b *junitapi.JUnitTestCase) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if (a.FailureOutput == nil) != (b.FailureOutput == nil) {
		return false
	}
	if a.FailureOutput != nil && a.FailureOutput.Output != b.FailureOutput.Output {
		return false
	}
	if (a.SkipMessage == nil) != (b.SkipMessage == nil) {
		return false
	}
	if a.SkipMessage != nil && a.SkipMessage.Message != b.SkipMessage.Message {
		return false
	}
	return true
}

func formatJUnits(junits []*junitapi.JUnitTestCase) string {
	s := "["
	for i, j := range junits {
		if i > 0 {
			s += ", "
		}
		s += formatJUnit(j)
	}
	return s + "]"
}

func formatJUnit(j *junitapi.JUnitTestCase) string {
	if j == nil {
		return "<nil>"
	}
	s := j.Name
	if j.FailureOutput != nil {
		s += "(FAIL)"
	} else if j.SkipMessage != nil {
		s += "(SKIP)"
	} else {
		s += "(PASS)"
	}
	return s
}
