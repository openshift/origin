package base

import (
	"fmt"

	"github.com/spf13/cobra"
)

type Cmd struct {
	Name             string
	ShortDescription string
	LongDescription  string
	Executor         *CmdExecutor
	Children         []Cmd
}

type CmdExecutor struct {
	Execute func(string, []string)
}

func InstallCommands(parent *cobra.Command, commands []Cmd) *cobra.Command {
	var installed *cobra.Command
	for _, cmd := range commands {
		installed = InstallCommand(parent, cmd.Name, cmd.Executor, cmd.ShortDescription, cmd.LongDescription)
		InstallCommands(installed, cmd.Children)
	}
	return installed
}

func InstallCommand(parent *cobra.Command, name string, executor *CmdExecutor, shortDescription string, longDescription string) *cobra.Command {
	if shortDescription == "" {
		shortDescription = fmt.Sprintf("Short description for '%s' command", name)
	}

	if longDescription == "" {
		longDescription = fmt.Sprintf("Long description for '%s' command", name)
	}

	command := &cobra.Command{
		Use:   name,
		Short: shortDescription,
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			if executor == nil {
				c.Help()
			} else {
				executor.Execute(c.Name(), args)
			}
		},
	}

	if parent != nil {
		parent.AddCommand(command)
	}

	return command
}
