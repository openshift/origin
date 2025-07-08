package filters

import (
	"fmt"
	"testing"
	"time"
)

func TestSkippedUntil(t *testing.T) {
	future := func() string { return time.Now().AddDate(0, 0, 5).Format("01022006") }
	past := func() string { return time.Now().AddDate(0, 0, -5).Format("01022006") }
	today := func() string { return time.Now().Format("01022006") }
	unrecognized := func() string { return time.Now().Format("01-02-2006") }

	tests := []struct {
		name     string
		testName string
		skipped  bool
	}{
		{
			name:     "no skipped until tag",
			testName: "[sig-api-machinery][Feature:APIServer] testing foo [Suite:openshift/...]",
			skipped:  false,
		},
		{
			name:     "skipped until tag does not specify date or bz",
			testName: "[sig-api-machinery] testing foo [SkippedUntil:] [Suite:openshift/conformance/parallel/minimal]",
			skipped:  false,
		},
		{
			name:     "skipped until tag has invalid date",
			testName: "[sig-api-machinery] testing foo [SkippedUntil:abcdefgh:blocker-bz/123456] [Suite:openshift/conformance/parallel/minimal]",
			skipped:  false,
		},
		{
			name:     "skipped until tag has unrecognized date format",
			testName: fmt.Sprintf("[sig-api-machinery] testing foo [SkippedUntil:%s:blocker-bz/123456] [Suite:openshift/...", unrecognized()),
			skipped:  false,
		},
		{
			name:     "skipped until with valid date in the past",
			testName: fmt.Sprintf("[sig-api-machinery] testing foo [SkippedUntil:%s:blocker-bz/123456] [Suite:openshift/...]", past()),
			skipped:  false,
		},
		{
			name:     "skipped until with today's date",
			testName: fmt.Sprintf("[sig-api-machinery] testing foo [SkippedUntil:%s:blocker-bz/123456] [Suite:openshift/...]", today()),
			skipped:  false,
		},
		{
			name:     "skipped until with a valid date in the future, but no blocker bz specified",
			testName: fmt.Sprintf("[sig-api-machinery] testing foo [SkippedUntil:%s:blocker-bz/] [Suite:openshift]", future()),
			skipped:  false,
		},
		{
			name:     "skipped until with a valid date in the future, blocker bz is malformed",
			testName: fmt.Sprintf("[sig-api-machinery] testing foo [SkippedUntil:%s:blocker-bz/123*] [Suite:openshift]", future()),
			skipped:  false,
		},
		{
			name:     "skipped until with a valid date in the future",
			testName: fmt.Sprintf("[sig-api-machinery] testing foo [SkippedUntil:%s:blocker-bz/123456] [Suite:openshift]", future()),
			skipped:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Logf("test name: %s", test.testName)
			got := shouldSkipUntil(test.testName)
			if test.skipped != got {
				t.Errorf("Expected: %v, but got: %v", test.skipped, got)
			}
		})
	}
}
