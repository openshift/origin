package setup

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCmdSetup(name string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Command '%s' (main)", name),
		Long:  fmt.Sprintf("Command '%s' (main)", name),
		Run: func(c *cobra.Command, args []string) {
		},
	}
}
