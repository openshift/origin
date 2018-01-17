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
}

func (o *TraceImportsFlags) ToOptions(out, errout io.Writer) (*TraceImportsOpts, error) {
	return &TraceImportsOpts{
		Roots:   o.Roots,
		UniqueB: o.UniqueB,
		Exclude: o.Exclude,

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
	cmd.Flags().StringSliceVar(&flags.UniqueB, "unique-b", flags.UniqueB, "root packages of sub-trees contained within trees initially specified via --root. A set of transitive dependencies unique to these trees relative to the rest of the dependency graph is returned.")
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

	if len(o.UniqueB) > 0 {
		// determine roots
		knownRoots := map[graph.Node]bool{}
		for _, n := range g.Nodes() {
			if len(g.To(n)) > 0 {
				continue
			}

			knownRoots[n] = true
		}

		for _, rootName := range o.UniqueB {
			root := nodeByName(g, nodes, rootName)
			if root == nil {
				return fmt.Errorf("--shared root path %q not found in dependency graph", rootName)
			}

			g.RemoveNode(root)
		}

		// find orphaned nodes
		//orphaned := findOrphans(g, knownRoots)
		// TODO: build graph - highlight orphaned nodes

	}

	if len(o.OutputFormat) > 0 {
		return o.outputGraph(g)
	}

	return nil
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

func findOrphans(g graph.Directed, knownRoots map[graph.Node]bool) []graph.Node {
	orphans := []graph.Node{}

	for _, node := range g.Nodes() {
		if len(g.To(node)) > 0 {
			continue
		}
		if _, exists := knownRoots[node]; exists {
			continue
		}

		orphans = append(orphans, node)
	}

	return orphans
}
