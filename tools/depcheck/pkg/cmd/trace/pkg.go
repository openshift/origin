package trace

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gonum/graph/concrete"

	depgraph "github.com/openshift/origin/tools/depcheck/pkg/graph"
)

var (
	// Matches standard goimport format for a package.
	//
	// The following formats will successfully match a valid import path:
	//   - host.tld/repo/pkg
	//   - foo.bar/baz
	//
	// The following formats will fail to match an import path:
	//   - company.com
	//   - company/missing/tld
	//   - fmt
	//   - encoding/json
	baseRepoRegex = regexp.MustCompile("[a-zA-Z0-9]+\\.([a-z0-9])+\\/.+")
)

type Package struct {
	Dir         string
	ImportPath  string
	Imports     []string
	TestImports []string
}

type PackageList struct {
	Packages []Package
}

func (p *PackageList) Add(pkg Package) {
	p.Packages = append(p.Packages, pkg)
}

// BuildGraph receives a list of Go packages and constructs a dependency graph from it.
// Any core library dependencies (fmt, strings, etc.) are not added to the graph.
// Any packages whose import path is contained within a list of "excludes" are not added to the graph.
// Returns a directed graph and a map of package import paths to node ids, or an error.
func BuildGraph(packages *PackageList, excludes []string) (*depgraph.MutableDirectedGraph, error) {
	g := depgraph.NewMutableDirectedGraph(concrete.NewDirectedGraph())

	// contains the subset of packages from the set of given packages (and their immediate dependencies)
	// that will actually be included in our graph - any packages in the excludes slice, or that do not
	// do not match the baseRepoRegex pattern will be filtered out from this collection.
	filteredPackages := []Package{}

	// add nodes to graph
	for _, pkg := range packages.Packages {
		if isExcludedPath(pkg.ImportPath, excludes) {
			continue
		}
		if !isValidPackagePath(pkg.ImportPath) {
			continue
		}

		n := &depgraph.Node{
			Id:         g.NewNodeID(),
			UniqueName: pkg.ImportPath,
			LabelName:  labelNameForNode(pkg.ImportPath),
		}
		err := g.AddNode(n)
		if err != nil {
			return nil, err
		}

		filteredPackages = append(filteredPackages, pkg)
	}

	// add edges
	for _, pkg := range filteredPackages {
		from, exists := g.NodeByName(pkg.ImportPath)
		if !exists {
			return nil, fmt.Errorf("expected node for package %q was not found in graph", pkg.ImportPath)
		}

		for _, dependency := range append(pkg.Imports, pkg.TestImports...) {
			if isExcludedPath(dependency, excludes) {
				continue
			}
			if !isValidPackagePath(dependency) {
				continue
			}

			to, exists := g.NodeByName(dependency)
			if !exists {
				return nil, fmt.Errorf("expected child node for dependency %q was not found in graph", dependency)
			}

			if g.HasEdgeFromTo(from, to) {
				continue
			}

			g.SetEdge(concrete.Edge{
				F: from,
				T: to,
			}, 0)
		}
	}

	return g, nil
}

func isExcludedPath(path string, excludes []string) bool {
	for _, exclude := range excludes {
		if strings.HasPrefix(path, exclude) {
			return true
		}
	}

	return false
}

// labelNameForNode trims vendored paths of their full /vendor/ path
func labelNameForNode(importPath string) string {
	segs := strings.Split(importPath, "/vendor/")
	if len(segs) > 1 {
		return segs[1]
	}

	return importPath
}

func isValidPackagePath(path string) bool {
	return baseRepoRegex.Match([]byte(path))
}
