package graph

import (
	"testing"
)

type TestPackageList struct {
	*PackageList
}

func (p *TestPackageList) ImportPaths() []string {
	importPaths := []string{}
	for _, pkg := range p.PackageList.Packages {
		importPaths = append(importPaths, pkg.ImportPath)
	}

	return importPaths
}

var pkgs = &TestPackageList{
	&PackageList{
		Packages: []Package{
			{
				Dir:        "/path/to/github.com/test/repo/root",
				ImportPath: "github.com/test/repo/root",
				Imports: []string{
					"github.com/test/repo/pkg/one",
				},
			},
			{
				Dir:        "/path/to/github.com/test/repo/pkg/one",
				ImportPath: "github.com/test/repo/pkg/one",
				Imports: []string{
					"github.com/test/repo/pkg/two",
					"github.com/test/repo/pkg/three",
					"github.com/test/repo/pkg/depends_on_fmt",
				},
			},
			{
				Dir:        "/path/to/github.com/test/repo/pkg/two",
				ImportPath: "github.com/test/repo/pkg/two",
				Imports: []string{
					"github.com/test/repo/vendor/github.com/testvendor/vendor_one",
				},
			},
			{
				Dir:        "/path/to/github.com/test/repo/pkg/three",
				ImportPath: "github.com/test/repo/pkg/three",
				Imports: []string{
					"github.com/test/repo/shared/shared_one",
				},
			},
			{
				Dir:        "/path/to/github.com/test/repo/pkg/depends_on_fmt",
				ImportPath: "github.com/test/repo/pkg/depends_on_fmt",
				Imports: []string{
					"fmt",
					"github.com/test/repo/unique/unique_nonvendored_one",
				},
			},
			{
				Dir:        "/path/to/github.com/test/repo/unique/unique_nonvendored_one",
				ImportPath: "github.com/test/repo/unique/unique_nonvendored_one",
				Imports:    []string{},
			},
			{
				Dir:        "/path/to/github.com/test/repo/shared/shared_one",
				ImportPath: "github.com/test/repo/shared/shared_one",
				Imports:    []string{},
			},
			{
				Dir:        "/path/to/github.com/test/repo/vendor/github.com/testvendor/vendor_one",
				ImportPath: "github.com/test/repo/vendor/github.com/testvendor/vendor_one",
				Imports: []string{
					"github.com/test/repo/unique/unique_vendor_one",
					"github.com/test/repo/shared/shared_one",
				},
			},
			{
				Dir:        "/path/to/github.com/test/repo/unique/unique_vendor_one",
				ImportPath: "github.com/test/repo/unique/unique_vendor_one",
				Imports:    []string{},
			},

			// simulate a package that is not brought in through any of the repo entrypoints
			// ("github.com/test/repo/root" in this case) but exists in the codebase
			// because another package that is part of its repo is a transitive dependency
			// of one of the main repo's entrypoints.
			{
				Dir:        "/path/to/github.com/test/repo/unique/unique_vendor_two",
				ImportPath: "github.com/test/repo/unique/unique_vendor_two",
				Imports: []string{
					"github.com/test/repo/no/node/should/exist/for/this/pkg",
				},
			},
		},
	},
}

// pkgsWithNoNodes is a map containing importPaths for packages
// that are not expected to have a node in the dependency graph
var pkgsWithNoNodes = map[string]bool{
	"github.com/test/repo/no/node/should/exist/for/this/pkg": true,
}

func shouldHaveNode(name string) bool {
	_, exists := pkgsWithNoNodes[name]
	return !exists
}

func TestBuildGraphCreatesExpectedNodesAndEdges(t *testing.T) {
	invalidImports := map[string]bool{
		"fmt": true,
	}

	opts := GraphOptions{
		Packages: pkgs.PackageList,
		Roots:    pkgs.ImportPaths(),
	}

	g, err := opts.BuildGraph()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Nodes()) != len(pkgs.Packages) {
		t.Fatalf("node count mismatch. Expected %v nodes but got %v.", len(pkgs.Packages), len(g.Nodes()))
	}

	for _, pkg := range pkgs.Packages {
		from, exists := g.NodeByName(pkg.ImportPath)
		if !exists || !g.Has(from) {
			t.Fatalf("expected node with name to exist for given package with ImportPath %q", pkg.ImportPath)
		}

		for _, dep := range pkg.Imports {
			if _, skip := invalidImports[dep]; skip {
				continue
			}

			to, exists := g.NodeByName(dep)
			if !shouldHaveNode(dep) {
				if exists {
					t.Fatalf("expected node with name %q to not exist", dep)
				}

				continue
			}

			if !exists || !g.Has(to) {
				t.Fatalf("expected node with name %q to exist", dep)
			}

			if !g.HasEdgeFromTo(from, to) {
				t.Fatalf("expected edge to exist between nodes %v and %v", from, to)
			}
		}
	}
}

func TestBuildGraphExcludesNodes(t *testing.T) {
	excludes := []string{
		"github.com/test/repo/pkg/three",
		"github.com/test/repo/pkg/depends_on_fmt",
	}

	opts := GraphOptions{
		Packages: pkgs.PackageList,
		Roots:    pkgs.ImportPaths(),
		Excludes: excludes,
	}

	g, err := opts.BuildGraph()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, n := range g.Nodes() {
		node, ok := n.(*Node)
		if !ok {
			t.Fatalf("expected node to be of type *depgraph.Node")
		}

		for _, exclude := range excludes {
			if node.UniqueName == exclude {
				t.Fatalf("expected node with unique name %q to have been excluded from the graph", node.UniqueName)
			}
		}
	}

}

func TestPackagesWithInvalidPathsAreOmitted(t *testing.T) {
	pkgList := &TestPackageList{
		&PackageList{
			Packages: []Package{
				{
					Dir:        "/path/to/github.com/test/repo/invalid",
					ImportPath: "invalid/import/path1",
					Imports: []string{
						"fmt",
						"invalid.import.path2",
						"invalid.import.path3",
					},
				},
				{
					Dir:        "/path/to/github.com/test/repo/invalid",
					ImportPath: "invalid.import.path2",
					Imports: []string{
						"net",
						"encoding/json",
					},
				},
				{
					Dir:        "/path/to/github.com/test/repo/invalid",
					ImportPath: "invalid3",
				},
			},
		},
	}

	opts := GraphOptions{
		Packages: pkgList.PackageList,
		Roots:    pkgList.ImportPaths(),
	}

	g, err := opts.BuildGraph()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Nodes()) != 0 {
		t.Fatalf("expected no nodes to have been created for an invalid package list. Saw %v unexpected nodes.", len(g.Nodes()))
	}
}

func TestLabelNamesForVendoredNodes(t *testing.T) {
	pkgList := &TestPackageList{
		&PackageList{
			Packages: []Package{
				{
					Dir:        "/path/to/github.com/test/repo/vendor/github.com/testvendor/vendor_one",
					ImportPath: "github.com/test/repo/vendor/github.com/testvendor/vendor_one",
				},
			},
		},
	}

	expectedLabelName := "github.com/testvendor/vendor_one"

	opts := GraphOptions{
		Packages: pkgList.PackageList,
		Roots:    pkgList.ImportPaths(),
	}

	g, err := opts.BuildGraph()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Nodes()) != 1 {
		t.Fatalf("expected graph of size 1, but got graph with %v nodes", len(g.Nodes()))
	}

	node, ok := g.Nodes()[0].(*Node)
	if !ok {
		t.Fatalf("expected node %v to be of type *depgraph.Node", node)
	}

	actualLabelName := labelNameForNode(node.UniqueName)
	if actualLabelName != expectedLabelName {
		t.Fatalf("expected node label name to be %q but was %q", expectedLabelName, actualLabelName)
	}
}
