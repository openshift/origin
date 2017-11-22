package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/openshift/source-to-image/pkg/version"
)

// NewCmdVersion implements the S2i cli version command.
func NewCmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display version",
		Long:  "Display version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("s2i %v\n", version.Get())
		},
	}
}
