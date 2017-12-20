package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
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
	return cmd
}
