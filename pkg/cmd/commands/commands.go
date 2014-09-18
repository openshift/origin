package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func InstallCommand(cmd *cobra.Command, name string, executor func(string, []string), descriptions ...string) {
	var shortDescription = fmt.Sprintf("Short description for '%s' command", name)
	var longDescription = fmt.Sprintf("Long description for '%s' command", name)

	if len(descriptions) > 0 {
		shortDescription = descriptions[0]
	}

	if len(descriptions) > 1 {
		longDescription = descriptions[1]
	}

	cmd.AddCommand(&cobra.Command{
		Use:   name,
		Short: shortDescription,
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			executor(c.Name(), args)
		},
	})
}
