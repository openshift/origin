package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"strings"

	"github.com/spf13/cobra"

	"github.com/golang/glog"

	"github.com/gonum/graph/concrete"
)

// GraphOptions contains values necessary to create a dependency graph.
type GraphOptions struct {
	// Packages is the caculated list of traversed go packages to use to create the graph
	Packages *PackageList

	// Roots is a list of go-import paths containing the total set of traversed
	// packages, from the given set of Entrypoints.
	Roots []string
	// Excludes are package go-import paths to ignore while traversing a directory structure
	Excludes []string
	// Filters are package go-import paths. Any package paths nested under these are truncated.
	Filters []string
}

func (o *GraphOptions) Complete() error {
	return nil
}

func (o *GraphOptions) Validate() error {
	if o.Packages == nil || len(o.Packages.Packages) == 0 {
		return errors.New("a list of Go Packages is required in order to build the graph")
	}

	return nil
}

// BuildGraph receives a list of Go packages and constructs a dependency graph from it.\
// Each package's ImportPath and non-transitive (immediate) imports are
// filtered and aggregated. A package is filtered based on whether its ImportPath
// matches an accepted pattern defined in the set of validPackages.
// Any core library dependencies (fmt, strings, etc.) are not added to the graph.
// Any packages whose import path is contained within a list of "excludes" are not added to the graph.
// Returns a directed graph and a map of package import paths to node ids, or an error.
func (o *GraphOptions) BuildGraph() (*MutableDirectedGraph, error) {
	g := NewMutableDirectedGraph(o.Roots)

	// contains the subset of packages from the set of given packages (and their immediate dependencies)
	// that will actually be included in our graph - any packages in the excludes slice, or that do not
	// do not match the baseRepoRegex pattern will be filtered out from this collection.
	filteredPackages := []Package{}

	// add nodes to graph
	for _, pkg := range o.Packages.Packages {
		if isExcludedPath(pkg.ImportPath, o.Excludes) {
			continue
		}
		if !isValidPackagePath(pkg.ImportPath) {
			continue
		}

		n := &Node{
			Id:         g.NewNodeID(),
			UniqueName: pkg.ImportPath,
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
			if isExcludedPath(dependency, o.Excludes) {
				continue
			}
			if !isValidPackagePath(dependency) {
				continue
			}

			to, exists := g.NodeByName(dependency)
			if !exists {
				// if a package imports a dependency that we did not visit
				// while traversing the code tree, ignore it, as it is not
				// required for the root repository to build.
				glog.V(1).Infof("Skipping unvisited (missing) dependency %q, which is imported by package %q", dependency, pkg.ImportPath)
				continue
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

	// filter graph if any filters are specified
	if len(o.Filters) > 0 {
		var err error
		g, err = FilterPackages(g, o.Filters)
		if err != nil {
			return nil, err
		}
	}

	g.PruneOrphans()
	return g, nil
}

type GraphFlags struct {
	Filter  string
	Exclude string

	Openshift bool

	RepoImportPath string
	Entrypoints    []string
}

// calculate roots receives a set of entrypoints and traverses through
// the directory tree, returning a list of all reachable go packages.
// Excludes the vendor tree.
// Returns the list of calculated import paths or an error.
func (o *GraphFlags) calculateRoots() ([]string, error) {
	packages, err := getPackageMetadata(
		o.Entrypoints,
	)
	if err != nil {
		return nil, err
	}

	roots := []string{}
	for _, pkg := range packages.Packages {
		roots = append(roots, pkg.ImportPath)
	}

	return roots, nil
}

func (o *GraphFlags) ToOptions(out, errout io.Writer) (*GraphOptions, error) {
	opts := &GraphOptions{}

	if len(o.RepoImportPath) == 0 {
		return nil, errors.New("the go-import path for the repository must be specified via --root")
	}
	if len(o.Entrypoints) == 0 {
		return nil, errors.New("at least one entrypoint path must be provided")
	}
	if o.Openshift && (len(o.Exclude) > 0 || len(o.Filter) > 0) {
		return nil, errors.New("--exclude or --filter are mutually exclusive with --openshift")
	}

	// sanitize user-provided entrypoints
	o.Entrypoints = ensureEntrypointPrefix(o.Entrypoints, o.RepoImportPath)

	// calculate go package info from given set of entrypoints
	packages, err := getPackageMetadata(
		ensureVendorEntrypoint(o.Entrypoints, o.RepoImportPath),
	)
	if err != nil {
		return nil, err
	}

	opts.Packages = packages

	// calculate non-vendor trees
	roots, err := o.calculateRoots()
	if err != nil {
		return nil, err
	}
	opts.Roots = roots

	// set openshift defaults
	if o.Openshift {
		opts.Excludes = getOpenShiftExcludes()
		filters, err := getOpenShiftFilters()
		if err != nil {
			return nil, err
		}

		opts.Filters = filters
		return opts, nil
	}

	if len(o.Exclude) > 0 {
		f, err := ioutil.ReadFile(o.Exclude)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(f, &opts.Excludes)
		if err != nil {
			return nil, err
		}
	}

	if len(o.Filter) > 0 {
		f, err := ioutil.ReadFile(o.Filter)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(f, &opts.Filters)
		if err != nil {
			return nil, err
		}
	}

	return opts, nil
}

func (o *GraphFlags) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&o.Openshift, "openshift", o.Openshift, "generate and use OpenShift-specific lists of excluded packages and filters.")
	cmd.Flags().StringVar(&o.RepoImportPath, "root", o.RepoImportPath, "Go import-path of repository to analyze (e.g. github.com/openshift/origin)")
	cmd.Flags().StringSliceVar(&o.Entrypoints, "entry", o.Entrypoints, "filepaths for packages within the specified --root relative to the repo's import path (e.g. ./cmd/...). Paths ending in an ellipsis (...) are traversed recursively.")
	cmd.Flags().StringVarP(&o.Exclude, "exclude", "e", "", "optional path to json file containing a list of import-paths of packages within the specified repository to recursively exclude.")
	cmd.Flags().StringVarP(&o.Filter, "filter", "c", "", "optional path to json file containing a list of import-paths of packages to collapse sub-packages into.")
}

func isExcludedPath(path string, excludes []string) bool {
	for _, exclude := range excludes {
		if strings.HasPrefix(path, exclude) {
			return true
		}
	}

	return false
}

// ensureEntrypointPrefix receives a list of paths and ensures
// that each path is prefixed by the repo's go-import path:
//   ["./cmd"] -> ["github.com/openshift/origin/cmd"]
func ensureEntrypointPrefix(entrypoints []string, prefix string) []string {
	for idx, entry := range entrypoints {
		if strings.HasPrefix(entry, prefix) {
			continue
		}

		entrypoints[idx] = path.Join(prefix, entry)
	}

	return entrypoints
}

// ensureVendorEntrypoint receives a list of paths and ensures that
// at least one of those paths is a go-import path to the repo's vendor directory
func ensureVendorEntrypoint(entrypoints []string, prefix string) []string {
	hasVendor := false
	for _, entry := range entrypoints {
		if strings.HasSuffix(path.Clean(entry), "/vendor") {
			hasVendor = true
			break
		}
	}
	if !hasVendor {
		vendor := ensureEntrypointPrefix([]string{"vendor"}, prefix)
		vendor = ensureEntrypointEllipsis(vendor)
		entrypoints = append(entrypoints, vendor[0])
	}

	return entrypoints
}

// ensureEntrypointEllipsis receives a list of paths
// and ensures that each path ends in an ellipsis (...).
func ensureEntrypointEllipsis(entypoints []string) []string {
	parsedRoots := []string{}
	for _, entry := range entypoints {
		if strings.HasSuffix(entry, "...") {
			parsedRoots = append(parsedRoots, entry)
			continue
		}

		slash := ""
		if string(entry[len(entry)-1]) != "/" {
			slash = "/"
		}
		entry = strings.Join([]string{entry, slash, "..."}, "")
		parsedRoots = append(parsedRoots, entry)
	}

	return parsedRoots
}
