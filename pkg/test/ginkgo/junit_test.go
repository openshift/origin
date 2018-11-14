package ginkgo

import "testing"

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
