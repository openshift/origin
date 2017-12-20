package cmd

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
	"github.com/gonum/graph/concrete"
	"github.com/gonum/graph/encoding/dot"
)

var traceImportsExample = `# create a dependency graph
%[1]s trace --root=pkg/one --root=pkg/two

# create a dependency graph and output in "dot" format
%[1]s trace --root=pkg/one --output=dot
`

var (
	validPackages = []string{
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
	validDepPrefixes = []string{"github.com/openshift/origin"}
)

type TraceImportsOpts struct {
	BaseDir string
	Roots   []string

	OutputFormat string

	Out    io.Writer
	ErrOut io.Writer
}

type TraceImportsFlags struct {
	OutputFormat string
	Roots        []string
}

func (o *TraceImportsFlags) ToOptions(out, errout io.Writer) (*TraceImportsOpts, error) {
	return &TraceImportsOpts{
		Roots: o.Roots,

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
	cmd.Flags().StringVarP(&flags.OutputFormat, "output", "o", "", "output generated dependency graph in specified format. One of: dot.")
	return cmd
}

func (o *TraceImportsOpts) Complete() error {
	o.Roots = ensureRecursiveRootFormat(o.Roots)
	return nil
}

func ensureRecursiveRootFormat(roots []string) []string {
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

type Node struct {
	id         int
	uniqueName string
	labelName  string
}

func (n Node) ID() int {
	return n.id
}

// DOTAttributes implements an attribute getter for the DOT encoding
func (n Node) DOTAttributes() []dot.Attribute {
	return []dot.Attribute{{Key: "label", Value: fmt.Sprintf("%q", n.labelName)}}
}

type PackageImports struct {
	Imports []string
}

type Package struct {
	ImportPath string
	Imports    []string
}

type PackageList struct {
	Packages []Package
}

func (p *PackageList) filterDeps(pkg *Package) {
	validImports := []string{}
	for _, dep := range pkg.Imports {
		for _, valid := range validDepPrefixes {
			if strings.HasPrefix(dep, valid) {
				validImports = append(validImports, dep)
				break
			}
		}
	}

	pkg.Imports = validImports
}

// TODO: provide a way for consumers of this command
// to specify a differentiation between vendored and
// non-vendored dependencies
func (p *PackageList) FilteredAdd(pkg Package) {
	if !strings.Contains(pkg.ImportPath, "/vendor/") {
		p.filterDeps(&pkg)
		p.Add(pkg)
		return
	}

	for _, valid := range validPackages {
		if strings.Contains(pkg.ImportPath, valid) {
			p.filterDeps(&pkg)
			p.Add(pkg)
			return
		}
	}
}

func (p *PackageList) Add(pkg Package) {
	p.Packages = append(p.Packages, pkg)
}

// Run execs `go list` on all package entrypoints specified through --root.
// Each package's ImportPath and non-transitive (immediate) imports are
// filtered and aggregated. A package is filtered based on whether its ImportPath
// matches an accepted pattern defined in the set of validPackages.
// Each aggregated package becomes a node in a generated dependency graph.
// An edge is created between a package and each of its Imports.
func (o *TraceImportsOpts) Run() error {
	args := []string{"list", "--json"} // "./pkg/...", "./vendor/...", "./cmd/.."
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

			list.FilteredAdd(pkg)
		}
	}(pkgs)

	if err := golist.Run(); err != nil {
		return err
	}

	g := o.buildGraph(pkgs)
	if len(o.OutputFormat) > 0 {
		return o.outputGraph(g)
	}

	return nil
}

func labelNameForNode(importPath string) string {
	segs := strings.Split(importPath, "/vendor/")
	if len(segs) > 1 {
		return segs[1]
	}

	return importPath
}

func (o *TraceImportsOpts) buildGraph(packages *PackageList) graph.Directed {
	g := concrete.NewDirectedGraph()
	nodeIdsByName := map[string]int{}

	// add nodes to graph
	for _, pkg := range packages.Packages {
		n := Node{
			id:         g.NewNodeID(),
			uniqueName: pkg.ImportPath,
			labelName:  labelNameForNode(pkg.ImportPath),
		}
		g.AddNode(graph.Node(n))
		nodeIdsByName[pkg.ImportPath] = n.ID()
	}

	// add edges
	for _, pkg := range packages.Packages {
		nid, exists := nodeIdsByName[pkg.ImportPath]
		if !exists {
			continue
		}

		from := g.Node(nid)
		if from == nil {
			continue
		}

		for _, dependency := range pkg.Imports {
			nid, exists := nodeIdsByName[dependency]
			if !exists {
				continue
			}

			to := g.Node(nid)
			if to == nil {
				continue
			}

			g.SetEdge(concrete.Edge{
				F: from,
				T: to,
			}, 0)
		}
	}

	return g
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
