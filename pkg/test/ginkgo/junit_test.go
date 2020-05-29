package ginkgo

import (
	"testing"
	"time"
)

func Test_lastLines(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		max     int
		matches []string
		want    string
	}{
		{output: "", max: 0, want: ""},
		{output: "", max: 1, want: ""},
		{output: "test", max: 1, want: "test"},
		{output: "test\n", max: 1, want: "test"},
		{output: "test\nother", max: 1, want: "other"},
		{output: "test\nother\n", max: 1, want: "other"},
		{output: "test\nother\n", max: 2, want: "test\nother"},
		{output: "test\nother\n", max: 3, want: "test\nother"},
		{output: "test\n\n\nother\n", max: 2, want: "test\n\n\nother"},

		{output: "test\n\n\nother and stuff\n", max: 2, matches: []string{"other"}, want: "other and stuff"},
		{output: "test\n\n\nother\n", max: 2, matches: []string{"test"}, want: "test\n\n\nother"},
		{output: "test\n\n\nother\n", max: 1, matches: []string{"test"}, want: "other"},
		{output: "test\ntest\n\n\nother\n", max: 10, matches: []string{"test"}, want: "test\n\n\nother"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lastLinesUntil(tt.output, tt.max, tt.matches...); got != tt.want {
				t.Errorf("lastLines() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_renderJUnitReport(t *testing.T) {
	tests := []*testCase{
		{
			name:   "Testing a failure",
			failed: true,
			out:    []byte("the main widget exploded"),
		},
		{
			name:    "Testing a success",
			success: true,
		},
	}

	additionalResult := &JUnitTestCase{
		Name:      "Additional testing",
		SystemOut: "system out bla, bla, bla",
		Duration:  3,
		FailureOutput: &FailureOutput{
			Message: "the additional widget exploded",
			Output:  "bing! bang! boom!",
		},
		Properties: &JUnitProperties{
			Properties: []JUnitProperty{
				{
					Name:  "weight",
					Value: "informer",
				},
			},
		},
	}

	data, err := renderJUnitReport("openshift-tests", tests, 4*time.Second, additionalResult)
	if err != nil {
		t.Fatal(err)
	}

	expected := `<testsuite name="openshift-tests" tests="3" skipped="0" failures="2" time="4"><properties><property name="TestVersion" value="unknown"></property></properties><testcase name="Testing a failure" time="0"><failure>the main widget exploded</failure><system-out>the main widget exploded</system-out><properties><property name="weight" value="failure"></property></properties></testcase><testcase name="Testing a success" time="0"></testcase><testcase name="Additional testing" time="3"><failure message="the additional widget exploded">bing! bang! boom!</failure><system-out>system out bla, bla, bla</system-out><properties><property name="weight" value="informer"></property></properties></testcase></testsuite>`
	if string(data) != expected {
		t.Fatalf("unexpected report:\n\n%s\n\nexpected:\n\n%s", string(data), expected)
	}
}
