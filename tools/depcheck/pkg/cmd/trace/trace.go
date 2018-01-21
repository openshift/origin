package trace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gonum/graph"
	"github.com/gonum/graph/encoding/dot"
)

var traceImportsExample = `# create a dependency graph
%[1]s trace --root=pkg/one --root=pkg/two

# create a dependency graph and output in "dot" format
%[1]s trace --root=pkg/one --output=dot
`

type TraceImportsOpts struct {
	BaseDir string
	Roots   []string
	Exclude []string
	Shared  bool

	// special operations
	UniqueB []string

	OutputFormat string

	Out    io.Writer
	ErrOut io.Writer
}

type TraceImportsFlags struct {
	OutputFormat string
	Roots        []string
	UniqueB      []string
	Exclude      []string
	Shared       bool
}

func (o *TraceImportsFlags) ToOptions(out, errout io.Writer) (*TraceImportsOpts, error) {
	return &TraceImportsOpts{
		Roots:   o.Roots,
		UniqueB: o.UniqueB,
		Exclude: o.Exclude,
		Shared:  o.Shared,

		OutputFormat: o.OutputFormat,

		Out:    out,
		ErrOut: errout,
	}, nil
}

func NewCmdTraceImports(parent string, out, errout io.Writer) *cobra.Command {
	flags := &TraceImportsFlags{}

	cmd := &cobra.Command{
		Use:     "trace --root=pkg/one",
		Short:   "Creates and analyzes a dependency graph from a set of given root packages",
		Long:    "Creates and analyzes a dependency graph from a set of given root packages",
		Example: fmt.Sprintf(traceImportsExample, parent),
		RunE: func(c *cobra.Command, args []string) error {
			opts, err := flags.ToOptions(out, errout)
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

	cmd.Flags().StringSliceVar(&flags.Roots, "root", flags.Roots, "set of entrypoints for dependency trees used to generate a depedency graph.")
	cmd.Flags().StringSliceVar(&flags.Exclude, "exclude", flags.Exclude, "set of paths to recursively exclude when traversing the set of given entrypoints specified through --root.")
	cmd.Flags().StringVarP(&flags.OutputFormat, "output", "o", "", "output generated dependency graph in specified format. One of: dot.")
	cmd.Flags().StringSliceVar(&flags.UniqueB, "subroot", flags.UniqueB, "root packages of sub-trees contained within trees initially specified via --root. A set of transitive dependencies unique to these trees relative to the rest of the dependency graph is returned.")
	cmd.Flags().BoolVar(&flags.Shared, "shared", flags.Shared, "indicates whether to include union of dependencies between --subroot trees and the rest of the graph in the final analysis.")
	return cmd
}

func (o *TraceImportsOpts) Complete() error {
	o.Roots = expandRecursePackages(o.Roots)
	return nil
}

// expandRecursePackages receives a list of root packages specified
// via --root, and ensures that each path ends in an ellipsis (...).
// This ensures that "go list" returns a recursive list of each root
// package's dependencies.
func expandRecursePackages(roots []string) []string {
	parsedRoots := []string{}
	for _, root := range roots {
		if strings.HasSuffix(root, "...") {
			parsedRoots = append(parsedRoots, root)
			continue
		}

		slash := ""
		if string(root[len(root)-1]) != "/" {
			slash = "/"
		}
		root = strings.Join([]string{root, slash, "..."}, "")
		parsedRoots = append(parsedRoots, root)
	}

	return parsedRoots
}

func (o *TraceImportsOpts) Validate() error {
	if len(o.Roots) == 0 {
		return errors.New("at least one root package must be provided")
	}
	if len(o.OutputFormat) != 0 && o.OutputFormat != "dot" {
		return fmt.Errorf("invalid output format provided: %s", o.OutputFormat)
	}

	return nil
}

// Run execs `go list` on all package entrypoints specified through --root.
// Each package's ImportPath and non-transitive (immediate) imports are
// filtered and aggregated. A package is filtered based on whether its ImportPath
// matches an accepted pattern defined in the set of validPackages.
// Each aggregated package becomes a node in a generated dependency graph.
// An edge is created between a package and each of its Imports.
func (o *TraceImportsOpts) Run() error {
	args := []string{"list", "--json"}
	golist := exec.Command("go", append(args, o.Roots...)...)

	r, w := io.Pipe()
	golist.Stdout = w
	golist.Stderr = os.Stderr

	pkgs := &PackageList{}
	go func(list *PackageList) {
		decoder := json.NewDecoder(r)
		for {
			var pkg Package
			err := decoder.Decode(&pkg)
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}

			list.Add(pkg)
		}
	}(pkgs)

	if err := golist.Run(); err != nil {
		return err
	}

	g, nodes, err := BuildGraph(pkgs, o.Exclude)
	if err != nil {
		return err
	}

	if len(o.OutputFormat) > 0 {
		return o.outputGraph(g)
	}

	if len(o.UniqueB) == 0 {
		return fmt.Errorf("at least one sub-tree root must be specified in order to perform a dependency analysis")
	}

	// determine roots
	knownRoots := map[graph.Node]bool{}
	for _, n := range g.Nodes() {
		if len(g.To(n)) > 0 {
			continue
		}

		knownRoots[n] = true
	}

	// determine unique set of A
	for root := range knownRoots {
		g.RemoveNode(root)
	}

	// roots of the provided subtrees within the dep graph
	subsetRoots := map[graph.Node]bool{}
	for _, rootName := range o.UniqueB {
		root := nodeByName(g, nodes, rootName)
		if root == nil {
			continue
		}

		subsetRoots[root] = true
	}

	// find unique set of nodes - not reachable from any known root
	unique := findUniqueSet(g, subsetRoots)
	fmt.Printf("Packages unique to me (%v):\n", knownRootsByName(knownRoots, nodes))
	for _, o := range unique {
		fmt.Printf("  - %v\n", nodeNameById(nodes, o.ID()))
	}
	fmt.Println()

	// determine unique set of B
	g2, _, err := BuildGraph(pkgs, o.Exclude)
	if err != nil {
		return err
	}

	for _, rootName := range o.UniqueB {
		root := nodeByName(g2, nodes, rootName)
		if root == nil {
			return fmt.Errorf("--shared root path %q not found in dependency graph", rootName)
		}

		g2.RemoveNode(root)
	}

	// find unique set of nodes - not reachable from any known root
	uniqueB := findUniqueSet(g2, knownRoots)
	fmt.Printf("Packages unique to you (%v):\n", o.UniqueB)
	for _, n := range uniqueB {
		fmt.Printf("  - %v\n", nodeNameById(nodes, n.ID()))
	}

	if !o.Shared {
		return nil
	}

	fmt.Println()

	uniqueSet := unionSetById(unique, uniqueB)
	// add roots to uniqueSet
	for n := range knownRoots {
		uniqueSet[n.ID()] = n
	}
	for n := range subsetRoots {
		uniqueSet[n.ID()] = n
	}

	// print out disjoint set
	sharedSet := map[graph.Node]bool{}
	fmt.Printf("Packages shared by %v and %v\n", knownRootsByName(knownRoots, nodes), o.UniqueB)
	for nodeName, n := range nodes {
		_, exists := uniqueSet[n]
		if exists {
			continue
		}

		node := nodeByName(g, nodes, nodeName)
		if node == nil {
			continue
		}

		sharedSet[node] = true
		fmt.Printf("  - %v\n", nodeNameById(nodes, n))
	}

	// TODO: print $depth-level packages responsible for bringing in packages in the sharedSet
	return nil
}

func knownRootsByName(nodeMap map[graph.Node]bool, nodes map[string]int) []string {
	names := []string{}
	for n := range nodeMap {
		names = append(names, nodeNameById(nodes, n.ID()))
	}

	return names
}

func unionSetById(A []graph.Node, B []graph.Node) map[int]graph.Node {
	union := map[int]graph.Node{}
	for _, n := range A {
		union[n.ID()] = n
	}
	for _, n := range B {
		union[n.ID()] = n
	}

	return union
}

func nodeNameById(nodes map[string]int, nodeId int) string {
	for k, v := range nodes {
		if v == nodeId {
			return k
		}
	}

	return ""
}

func (o *TraceImportsOpts) outputGraph(g graph.Directed) error {
	if o.OutputFormat != "dot" {
		return fmt.Errorf("invalid output format provided: %s", o.OutputFormat)
	}

	data, err := dot.Marshal(g, fmt.Sprintf("%q", strings.Join(o.Roots, ", ")), "", " ", false)
	if err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "%v\n", string(data))
	return nil
}

func nodeByName(g graph.Directed, nodes map[string]int, nodeName string) graph.Node {
	nid, exists := nodes[nodeName]
	if !exists {
		return nil
	}

	for _, n := range g.Nodes() {
		if n.ID() == nid {
			return n
		}
	}

	return nil
}

func findUniqueSet(g graph.Directed, knownRoots map[graph.Node]bool) []graph.Node {
	unique := []graph.Node{}

	for _, node := range g.Nodes() {
		for root := range knownRoots {
			if closureExists(g, root, node) {
				continue
			}

			unique = append(unique, node)
		}
	}

	return unique
}

// closureExists recursively determines whether or not a
// transitive closure exists from a given node A to a given node B.
// Returns a boolean true if B can be reached from A.
func closureExists(g graph.Directed, A graph.Node, B graph.Node) bool {
	if A.ID() == B.ID() {
		return true
	}

	toNodes := g.To(B)
	if len(toNodes) == 0 {
		return false
	}

	for _, n := range toNodes {
		if A.ID() == n.ID() {
			return true
		}
	}

	for _, n := range toNodes {
		if closureExists(g, A, n) {
			return true
		}
	}

	return false
}
