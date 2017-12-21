package trace

import (
	"fmt"
	"strings"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"

	depgraph "github.com/openshift/origin/tools/depcheck/pkg/graph"
)

var (
	// lets us distinguish core library deps
	knownVendorRoots = []string{
		"/vendor/bitbucket.org/",
		"/vendor/cloud.google.com",
		"/vendor/github.com",
		"/vendor/go4.org",
		"/vendor/golang.org",
		"/vendor/google.golang.org",
		"/vendor/gopkg.in",
		"/vendor/k8s.io/",
		"/vendor/vbom.ml",
	}
	baseRepo = "github.com/openshift/origin"
)

type Package struct {
	ImportPath  string
	Imports     []string
	TestImports []string
}

type PackageList struct {
	Packages []Package
}

// removeCoreLibraryImports receives a Package and filters any
// dependencies not part of the base repo or its vendor tree
func (p *PackageList) removeCoreLibraryImports(imports []string) []string {
	validImports := []string{}
	for _, dep := range imports {
		if !strings.Contains(dep, "/vendor/") && !strings.HasPrefix(dep, baseRepo) {
			continue
		}

		validImports = append(validImports, dep)
	}

	return validImports
}

// TODO: provide a way for consumers of this command
// to specify a differentiation between vendored and
// non-vendored dependencies
func (p *PackageList) FilteredAdd(pkg Package, excludes []string) {
	for _, exclude := range excludes {
		if strings.Contains(pkg.ImportPath, exclude) {
			return
		}
	}

	if !strings.Contains(pkg.ImportPath, "/vendor/") {
		pkg.Imports = p.removeCoreLibraryImports(pkg.Imports)
		pkg.TestImports = p.removeCoreLibraryImports(pkg.TestImports)
		p.Add(pkg)
		return
	}

	for _, valid := range knownVendorRoots {
		if strings.Contains(pkg.ImportPath, valid) {
			pkg.Imports = p.removeCoreLibraryImports(pkg.Imports)
			pkg.TestImports = p.removeCoreLibraryImports(pkg.TestImports)
			p.Add(pkg)
			return
		}
	}
}

func (p *PackageList) Add(pkg Package) {
	p.Packages = append(p.Packages, pkg)
}

func BuildGraph(packages *PackageList) (graph.Directed, error) {
	g := concrete.NewDirectedGraph()
	nodeIdsByName := map[string]int{}

	// add nodes to graph
	for _, pkg := range packages.Packages {
		n := depgraph.Node{
			Id:         g.NewNodeID(),
			UniqueName: pkg.ImportPath,
			LabelName:  labelNameForNode(pkg.ImportPath),
		}
		g.AddNode(graph.Node(n))
		nodeIdsByName[pkg.ImportPath] = n.ID()
	}

	// add edges
	for _, pkg := range packages.Packages {
		nid, exists := nodeIdsByName[pkg.ImportPath]
		if !exists {
			return nil, fmt.Errorf("expected package %q was not found", pkg.ImportPath)
		}

		from := g.Node(nid)
		if from == nil {
			return nil, fmt.Errorf("expected node with id %v for package %q was not found in graph", nid, pkg.ImportPath)
		}

		for _, dependency := range append(pkg.Imports, pkg.TestImports...) {
			nid, exists := nodeIdsByName[dependency]
			if !exists {
				return nil, fmt.Errorf("expected dependency %q for package %q was not found", dependency, pkg.ImportPath)
			}

			to := g.Node(nid)
			if to == nil {
				return nil, fmt.Errorf("expected child node %q with id %v was not found in graph", dependency, nid)
			}

			g.SetEdge(concrete.Edge{
				F: from,
				T: to,
			}, 0)
		}
	}

	return g, nil
}
