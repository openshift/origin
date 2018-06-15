package cmd

import (
	"testing"

	"github.com/openshift/origin/tools/depcheck/pkg/graph"
)

func newGraphOptions() *graph.GraphOptions {
	opts := &graph.GraphOptions{
		Packages: &graph.PackageList{
			Packages: []graph.Package{
				{
					ImportPath: "foo.com/bar/baz",
					Imports: []string{
						"foo.com/bar/baz/one",
						"foo.com/bar/baz/two",
					},
				},
				{
					ImportPath: "foo.com/bar/baz/one",
					Imports: []string{
						"foo.com/bar/baz/vendor/vendor.com/one",
						"foo.com/bar/baz/vendor/vendor.com/mine",
					},
				},
				{
					ImportPath: "foo.com/bar/baz/two",
					Imports: []string{
						"foo.com/bar/baz/vendor/vendor.com/ours",
					},
				},
				{
					ImportPath: "foo.com/bar/baz/vendor/vendor.com/one",
					Imports: []string{
						"foo.com/bar/baz/vendor/vendor.com/two",
						"foo.com/bar/baz/vendor/vendor.com/three",
					},
				},
				{
					ImportPath: "foo.com/bar/baz/vendor/vendor.com/two",
					Imports: []string{
						"fmt",
					},
				},
				{
					ImportPath: "foo.com/bar/baz/vendor/vendor.com/three",
					Imports: []string{
						"foo.com/bar/baz/vendor/vendor.com/ours",
						"foo.com/bar/baz/vendor/vendor.com/transitive_ours",
					},
				},
				{
					ImportPath: "foo.com/bar/baz/vendor/vendor.com/ours",
					Imports: []string{
						"foo.com/bar/baz/vendor/vendor.com/transitive_ours",
					},
				},
				{
					ImportPath: "foo.com/bar/baz/vendor/vendor.com/mine",
					Imports: []string{
						"fmt",
					},
				},
			},
		},
	}

	// add roots
	opts.Roots = []string{
		"foo.com/bar/baz",
		"foo.com/bar/baz/one",
		"foo.com/bar/baz/two",
	}

	return opts
}

func TestGraphAnalyzisCalculatesYoursMineOurs(t *testing.T) {
	opts := &AnalyzeOptions{
		GraphOptions: newGraphOptions(),
		Dependencies: []string{
			"foo.com/bar/baz/vendor/vendor.com/one",
		},
	}

	g, err := opts.GraphOptions.BuildGraph()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	yours, mine, ours, err := opts.calculateDependencies(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedYours := []string{
		"vendor.com/two",
		"vendor.com/three",
	}
	expectedMine := []string{
		"vendor.com/one",
		"vendor.com/mine",
	}
	expectedOurs := []string{
		"vendor.com/ours",
	}

	// transitive dependencies (that are also not unique to the vendor dep we are analyzing)
	expectedMissing := []string{
		"vendor.com/transitive_ours",
	}

	if len(yours) != len(expectedYours) {
		t.Errorf("node count mismatch; expecting %v \"yours\" dependencies, but got %v", len(expectedYours), len(yours))
	}
	if len(ours) != len(expectedOurs) {
		t.Errorf("node count mismatch; expecting %v \"ours\" dependencies, but got %v", len(expectedOurs), len(ours))
	}
	if len(mine) != len(expectedMine) {
		t.Errorf("node count mismatch; expecting %v \"mine\" dependencies, but got %v", len(expectedMine), len(mine))
	}

	for _, expected := range expectedOurs {
		found := false
		for _, actualOurs := range ours {
			if expected == actualOurs.String() {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected \"ours\" dependency %q was not found", expected)
		}
	}

	for _, expectedNode := range expectedYours {
		found := false
		for _, actualNode := range yours {
			if actualNode.String() == expectedNode {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected \"yours\" dependency %q was not found", expectedNode)
		}
	}

	for _, expected := range expectedMine {
		found := false
		for _, actualMine := range mine {
			if expected == actualMine.String() {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected \"ours\" dependency %q was not found", expected)
		}
	}

	for _, missing := range expectedMissing {
		found := false
		for _, actual := range append(yours, append(mine, ours...)...) {
			if missing == actual.String() {
				found = true
				break
			}
		}

		if found {
			t.Errorf("expecting %q to be missing, but was found in yours, mine, ours set", missing)
		}
	}
}
