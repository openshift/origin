package certs

import (
	"github.com/spf13/cobra"
)

func NewCommandCerts() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "certs",
		Short: "Create certificates",
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	cmd.AddCommand(NewCommandCreateKubeConfig())
	cmd.AddCommand(NewCommandCreateAllCerts())
	cmd.AddCommand(NewCommandCreateClientCert())
	cmd.AddCommand(NewCommandCreateNodeClientCert())
	cmd.AddCommand(NewCommandCreateServerCert())
	cmd.AddCommand(NewCommandCreateSignerCert())

	return cmd
}
