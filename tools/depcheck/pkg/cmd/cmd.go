package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/tools/depcheck/pkg/cmd/trace"
)

func NewCmdDepCheck(name string, out, errout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s (ARGUMENT) [OPTIONS]", name),
		Short:   "Gather information about a dependency tree.",
		Long:    "Modfify or gather information about a dependency tree.",
		Example: fmt.Sprintf(pinImportsExample, name),
		RunE: func(c *cobra.Command, args []string) error {
			c.SetOutput(errout)
			return c.Help()
		},
	}

	cmd.AddCommand(NewCmdPinImports(name, out, errout))
	cmd.AddCommand(trace.NewCmdTraceImports(name, out, errout))
	return cmd
}
