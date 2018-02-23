package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/gonum/graph"
	"github.com/gonum/graph/encoding/dot"

	depgraph "github.com/openshift/origin/tools/depcheck/pkg/graph"
)

var traceImportsExample = `# create a dependency graph
%[1]s trace --root=github.com/openshift/origin --entry="./cmd/..."

# create a dependency graph using OpenShift-specific settings
%[1]s trace --root=github.com/openshift/origin --entry="./cmd/..." --openshift

# create a dependency graph and output in "dot" format
%[1]s trace --root=github.com/openshift/origin --entry=pkg/foo/... --output=dot --openshift
`

type TraceOptions struct {
	GraphOptions *depgraph.GraphOptions

	outputGraphName string
	OutputFormat    string

	Out    io.Writer
	ErrOut io.Writer
}

type TraceFlags struct {
	GraphFlags *depgraph.GraphFlags

	OutputFormat string
}

func NewCmdTraceImports(parent string, out, errout io.Writer) *cobra.Command {
	traceFlags := &TraceFlags{
		GraphFlags:   &depgraph.GraphFlags{},
		OutputFormat: "dot",
	}

	cmd := &cobra.Command{
		Use:     "trace --root=github.com/openshift/origin --entry=pkg/foo/...",
		Short:   "Creates a dependency graph for a given repository",
		Long:    "Creates a dependency graph for a given repository, for every Go package reachable from a set of --entry points into the codebase",
		Example: fmt.Sprintf(traceImportsExample, parent),
		RunE: func(c *cobra.Command, args []string) error {
			opts, err := traceFlags.ToOptions(out, errout)
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

	traceFlags.GraphFlags.AddFlags(cmd)
	cmd.Flags().StringVarP(&traceFlags.OutputFormat, "output", "o", traceFlags.OutputFormat, "output generated dependency graph in specified format. One of: dot.")
	return cmd
}

func (o *TraceOptions) Complete() error {
	return o.GraphOptions.Complete()
}

func (o *TraceOptions) Validate() error {
	if err := o.GraphOptions.Validate(); err != nil {
		return err
	}

	if len(o.OutputFormat) > 0 && o.OutputFormat != "dot" {
		return fmt.Errorf("invalid output format provided: %s", o.OutputFormat)
	}

	return nil
}

func (o *TraceFlags) ToOptions(out, errout io.Writer) (*TraceOptions, error) {
	graphOpts, err := o.GraphFlags.ToOptions(out, errout)
	if err != nil {
		return nil, err
	}

	return &TraceOptions{
		GraphOptions: graphOpts,
		OutputFormat: o.OutputFormat,

		outputGraphName: o.GraphFlags.RepoImportPath,

		Out:    out,
		ErrOut: errout,
	}, nil
}

// Run creates a dependency graph using user-specified options.
// Outputs graph contents in the format specified.
func (o *TraceOptions) Run() error {
	g, err := o.GraphOptions.BuildGraph()
	if err != nil {
		return err
	}

	return o.outputGraph(g)
}

func (o *TraceOptions) outputGraph(g graph.Directed) error {
	if o.OutputFormat != "dot" {
		return fmt.Errorf("invalid output format provided: %s", o.OutputFormat)
	}

	data, err := dot.Marshal(g, fmt.Sprintf("%q", o.outputGraphName), "", " ", false)
	if err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "%v\n", string(data))
	return nil
}
