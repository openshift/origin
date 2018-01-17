package trace

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"

	depgraph "github.com/openshift/origin/tools/depcheck/pkg/graph"
)

var (
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
// Returns a directed acyclic graph and a map of package import paths to node ids, or an error.
func BuildGraph(packages *PackageList, excludes []string) (depgraph.MutableDirectedGraph, map[string]int, error) {
	g := concrete.NewDirectedGraph()
	nodeIdsByName := map[string]int{}

	filteredPackages := []Package{}

	// add nodes to graph
	for _, pkg := range packages.Packages {
		if isExcludedPath(pkg.ImportPath, excludes) {
			continue
		}

		n := depgraph.Node{
			Id:         g.NewNodeID(),
			UniqueName: pkg.ImportPath,
			LabelName:  labelNameForNode(pkg.ImportPath),
		}
		g.AddNode(graph.Node(n))
		nodeIdsByName[pkg.ImportPath] = n.ID()
		filteredPackages = append(filteredPackages, pkg)
	}

	// add edges
	for _, pkg := range filteredPackages {
		nid, exists := nodeIdsByName[pkg.ImportPath]
		if !exists {
			return nil, nil, fmt.Errorf("expected package %q was not found", pkg.ImportPath)
		}

		from := g.Node(nid)
		if from == nil {
			return nil, nil, fmt.Errorf("expected node with id %v for package %q was not found in graph", nid, pkg.ImportPath)
		}

		for _, dependency := range append(pkg.Imports, pkg.TestImports...) {
			if isExcludedPath(dependency, excludes) {
				continue
			}
			if !isValidPackagePath(dependency) {
				continue
			}

			nid, exists := nodeIdsByName[dependency]
			if !exists {
				return nil, nil, fmt.Errorf("expected dependency %q for package %q was not found", dependency, pkg.ImportPath)
			}

			to := g.Node(nid)
			if to == nil {
				return nil, nil, fmt.Errorf("expected child node %q with id %v was not found in graph", dependency, nid)
			}

			g.SetEdge(concrete.Edge{
				F: from,
				T: to,
			}, 0)
		}
	}

	return g, nodeIdsByName, nil
}

func isExcludedPath(path string, excludes []string) bool {
	for _, exclude := range excludes {
		if strings.HasPrefix(path, exclude) {
			return true
		}
	}

	return false
}

// labelNameForNode trims vendored paths of their
// full /vendor/ path
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
