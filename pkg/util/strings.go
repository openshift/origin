package util

import (
	"fmt"
	"sort"
	"strings"

	diff "github.com/sergi/go-diff/diffmatchpatch"
)

// UniqueStrings returns a sorted, uniquified slice of the specified strings
func UniqueStrings(strings []string) []string {
	m := make(map[string]bool, len(strings))
	for _, s := range strings {
		m[s] = true
	}

	i := 0
	strings = make([]string, len(m), len(m))
	for s := range m {
		strings[i] = s
		i++
	}

	sort.Strings(strings)
	return strings
}

// Diff returns the difference between two strings by using the
// diff-match-patch algorithm.
func Diff(previous, current string) string {
	patch := []string{}
	dmp := diff.New()

	t1, t2, t := dmp.DiffLinesToChars(previous, current)
	diffs := dmp.DiffMain(t1, t2, false)
	diffs = dmp.DiffCharsToLines(diffs, t)
	for _, d := range diffs {
		switch d.Type {
		case diff.DiffDelete:
			patch = append(patch, fmt.Sprintf("- %s", d.Text))
		case diff.DiffInsert:
			patch = append(patch, fmt.Sprintf("+ %s", d.Text))
		default:
			patch = append(patch, d.Text)
		}
	}
	return fmt.Sprintf("%s\n", strings.Join(patch, "\n"))
}
