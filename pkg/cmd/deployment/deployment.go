package deployment

import (
	"fmt"

	p "github.com/openshift/origin/pkg/cmd/util/printer"
	api "github.com/openshift/origin/pkg/deploy/api"
	"github.com/spf13/cobra"
)

var printer = p.TerminalPrinter{} // TODO: improve, we can think about things like FilePrinter, JsonPrinter, etc

// Root Deployment Command

func NewCommandDeployment(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Command '%s' (main)", name),
		Long:  fmt.Sprintf("Command '%s' (main)", name),
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}
	cmd.AddCommand(NewCommandDeploymentList("list"))
	cmd.AddCommand(NewCommandDeploymentShow("show"))
	cmd.AddCommand(NewCommandDeploymentUpdate("update"))
	cmd.AddCommand(NewCommandDeploymentRemove("remove"))
	return cmd
}

// Children Commands

func NewCommandDeploymentList(name string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Command '%s' (main)", name),
		Long:  fmt.Sprintf("Command '%s' (main)", name),
		Run: func(c *cobra.Command, args []string) {
			printer.Print("Fetching deployments ... ")

			items := api.DeploymentList{}.Items

			if len(items) == 0 {
				printer.Errorln("nothing found")

			} else {
				for _, d := range items {
					fmt.Printf("\n%s\t%s\n", d.ID, d.State)
				}

				printer.Successln("done")
			}
		},
	}
}

func NewCommandDeploymentShow(name string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Command '%s' (main)", name),
		Long:  fmt.Sprintf("Command '%s' (main)", name),
		Run: func(c *cobra.Command, args []string) {
		},
	}
}

func NewCommandDeploymentUpdate(name string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Command '%s' (main)", name),
		Long:  fmt.Sprintf("Command '%s' (main)", name),
		Run: func(c *cobra.Command, args []string) {
		},
	}
}

func NewCommandDeploymentRemove(name string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Command '%s' (main)", name),
		Long:  fmt.Sprintf("Command '%s' (main)", name),
		Run: func(c *cobra.Command, args []string) {
		},
	}
}
