package certs

import (
	"github.com/spf13/cobra"
)

func NewCommandAdmin() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Admin commands",
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
