package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gonum/graph/path"

	"github.com/openshift/origin/tools/depcheck/pkg/analyze"
	"github.com/openshift/origin/tools/depcheck/pkg/graph"
)

var analyzeImportsExample = `# analyze a dependency graph against one of its vendor packages
%[1]s analyze --root=github.com/openshift/origin --entry=pkg/foo/... --dep=github.com/openshift/origin/vendor/k8s.io/kubernetes

# analyze a dependency graph against one of its vendor packages using OpenShift defaults
%[1]s analyze --root=github.com/openshift/origin --entry=cmd/... --entry=pkg/... --entry=tools/... --entry=test/... --openshift --dep=github.com/openshift/origin/vendor/k8s.io/kubernetes
`

type AnalyzeOptions struct {
	GraphOptions *graph.GraphOptions

	// Packages to analyze
	Dependencies []string

	Out    io.Writer
	ErrOut io.Writer
}

type AnalyzeFlags struct {
	GraphFlags *graph.GraphFlags

	Dependencies []string
}

func (o *AnalyzeFlags) ToOptions(out, errout io.Writer) (*AnalyzeOptions, error) {
	graphOpts, err := o.GraphFlags.ToOptions(out, errout)
	if err != nil {
		return nil, err
	}

	return &AnalyzeOptions{
		GraphOptions: graphOpts,
		Dependencies: o.Dependencies,

		Out:    out,
		ErrOut: errout,
	}, nil
}

func NewCmdAnalyzeImports(parent string, out, errout io.Writer) *cobra.Command {
	analyzeFlags := &AnalyzeFlags{
		GraphFlags: &graph.GraphFlags{},
	}

	cmd := &cobra.Command{
		Use:     "analyze --root=github.com/openshift/origin --entry=pkg/foo/... --dep pkg/vendor/bar",
		Short:   "Creates and analyzes a dependency graph against a specified subpackage",
		Long:    "Creates and analyzes a dependency graph against a specified subpackage",
		Example: fmt.Sprintf(traceImportsExample, parent),
		RunE: func(c *cobra.Command, args []string) error {
			opts, err := analyzeFlags.ToOptions(out, errout)
			if err != nil {
				return err
			}

			if err := opts.Complete(); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Run(); err != nil {
				return err
			}

			return nil
		},
	}

	analyzeFlags.GraphFlags.AddFlags(cmd)
	cmd.Flags().StringSliceVarP(&analyzeFlags.Dependencies, "dep", "d", analyzeFlags.Dependencies, "import path of the dependency to analyze. Multiple --dep values may be provided.")
	return cmd
}

func (o *AnalyzeOptions) Complete() error {
	return o.GraphOptions.Complete()
}

func (o *AnalyzeOptions) Validate() error {
	if err := o.GraphOptions.Validate(); err != nil {
		return err
	}
	if len(o.Dependencies) == 0 {
		return errors.New("at least one --dep package must be specified")
	}

	return nil
}

func (o *AnalyzeOptions) Run() error {
	g, err := o.GraphOptions.BuildGraph()
	if err != nil {
		return err
	}

	return o.analyzeGraph(g)
}

// analyzeGraph receives a MutableDirectedGraph and outputs
// - "Yours": a list of every node in the set of dependencies unique to a target (--dep) node.
// - "Mine": a list of every node in the set of dependencies unique to the root nodes.
// - "Ours": a list of every node in the overlapping set between the "Yours" and "Mine" sets.
func (o *AnalyzeOptions) analyzeGraph(g *graph.MutableDirectedGraph) error {
	yours, mine, ours, err := o.calculateDependencies(g)
	if err != nil {
		return err
	}

	fmt.Printf("Analyzing a total of %v packages\n", len(g.Nodes()))
	fmt.Println()

	fmt.Printf("\"Yours\": %v dependencies exclusive to %q\n", len(yours), o.Dependencies)
	for _, n := range yours {
		fmt.Printf("    - %s\n", n)
	}
	fmt.Println()

	fmt.Printf("\"Mine\": %v direct (first-level) dependencies exclusive to the origin repo\n", len(mine))
	for _, n := range mine {
		fmt.Printf("    - %s\n", n)
	}
	fmt.Println()

	fmt.Printf("\"Ours\": %v shared dependencies between the origin repo and %v\n", len(ours), o.Dependencies)
	for _, n := range ours {
		fmt.Printf("    - %s\n", n)
	}

	return nil
}

func (o *AnalyzeOptions) calculateDependencies(g *graph.MutableDirectedGraph) ([]*graph.Node, []*graph.Node, []*graph.Node, error) {
	yoursRoots := []*graph.Node{}
	for _, dep := range o.Dependencies {
		n, exists := g.NodeByName(dep)
		if !exists {
			return nil, nil, nil, fmt.Errorf("unable to find dependency with import path %q", dep)
		}
		node, ok := n.(*graph.Node)
		if !ok {
			return nil, nil, nil, fmt.Errorf("expected node to analyze to be of type *graph.Node. Got: %v", n)
		}

		yoursRoots = append(yoursRoots, node)
	}

	yours := analyze.FindExclusiveDependencies(g, yoursRoots)

	// calculate root repo packages, as well as their
	// immediate vendor package dependencies
	unfilteredMine := map[int]*graph.Node{}
	for _, n := range g.Nodes() {
		node, ok := n.(*graph.Node)
		if !ok {
			return nil, nil, nil, fmt.Errorf("expected node to analyze to be of type *graph.Node. Got: %v", n)
		}
		if isVendorPackage(node) {
			continue
		}

		// obtain immediate vendor package deps from the current node
		// and aggregate those as well
		for _, v := range g.From(n) {
			if !isVendorPackage(v.(*graph.Node)) {
				continue
			}

			unfilteredMine[v.ID()] = v.(*graph.Node)
		}
	}

	mine := []*graph.Node{}
	ours := []*graph.Node{}
	for _, n := range unfilteredMine {
		// determine if the current origin node is reachable from any of the "yours" packages
		if isReachableFrom(g, yours, n) {
			ours = append(ours, n)
			continue
		}

		mine = append(mine, n)
	}

	return yours, mine, ours, nil
}

// isVendorPackage receives a *graph.Node and
// returns true if the given node's unique name is in the vendor path.
func isVendorPackage(n *graph.Node) bool {
	if strings.Contains(n.UniqueName, "/vendor/") {
		return true
	}

	return false
}

// isReachableFrom receives a set of root nodes and determines
// if a given destination node can be reached from any of them.
func isReachableFrom(g *graph.MutableDirectedGraph, roots []*graph.Node, dest *graph.Node) bool {
	for _, root := range roots {
		// no negative edge weights, use Dijkstra
		paths := path.DijkstraFrom(root, g)
		if pathTo, _ := paths.To(dest); len(pathTo) > 0 {
			return true
		}
	}

	return false
}
