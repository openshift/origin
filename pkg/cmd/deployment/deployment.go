package deployment

import (
	"fmt"

	"github.com/openshift/origin/pkg/cmd/util/formatting"
	api "github.com/openshift/origin/pkg/deploy/api"
	"github.com/spf13/cobra"
)

// Commands

func NewCommandDeployment(name string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Command '%s' (main)", name),
		Long:  fmt.Sprintf("Command '%s' (main)", name),
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}
}

func NewCommandDeploymentList(name string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Command '%s' (main)", name),
		Long:  fmt.Sprintf("Command '%s' (main)", name),
		Run: func(c *cobra.Command, args []string) {
			fmt.Printf("Fetching '%s' ... ", formatting.Strong("deployments"))

			items := api.DeploymentList{}.Items

			if len(items) == 0 {
				formatting.Printfln(formatting.Error("nothing found"))

			} else {
				for _, d := range items {
					fmt.Printf("\n%s\t%s\n", d.ID, d.State)
				}

				formatting.Printfln(formatting.Success("done"))
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
