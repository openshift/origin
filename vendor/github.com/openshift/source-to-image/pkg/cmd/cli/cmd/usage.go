package cmd

import (
	"github.com/spf13/cobra"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build/strategies/sti"
	cmdutil "github.com/openshift/source-to-image/pkg/cmd/cli/util"
	"github.com/openshift/source-to-image/pkg/docker"
	s2ierr "github.com/openshift/source-to-image/pkg/errors"
)

// NewCmdUsage implements the S2I cli usage command.
func NewCmdUsage(cfg *api.Config) *cobra.Command {
	oldScriptsFlag := ""
	oldDestination := ""

	usageCmd := &cobra.Command{
		Use:   "usage <image>",
		Short: "Print usage of the assemble script associated with the image",
		Long:  "Create and start a container from the image and invoke its usage script.",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				cmd.Help()
				return
			}

			cfg.Usage = true
			cfg.BuilderImage = args[0]

			if len(oldScriptsFlag) != 0 {
				log.Warning("DEPRECATED: Flag --scripts is deprecated, use --scripts-url instead")
				cfg.ScriptsURL = oldScriptsFlag
			}

			if len(cfg.BuilderPullPolicy) == 0 {
				cfg.BuilderPullPolicy = api.DefaultBuilderPullPolicy
			}
			if len(cfg.PreviousImagePullPolicy) == 0 {
				cfg.PreviousImagePullPolicy = api.DefaultPreviousImagePullPolicy
			}

			client, err := docker.NewEngineAPIClient(cfg.DockerConfig)
			s2ierr.CheckError(err)
			uh, err := sti.NewUsage(client, cfg)
			s2ierr.CheckError(err)
			err = uh.Show()
			s2ierr.CheckError(err)
		},
	}
	usageCmd.Flags().StringVarP(&(oldDestination), "location", "l", "",
		"Specify a destination location for untar operation")
	cmdutil.AddCommonFlags(usageCmd, cfg)
	return usageCmd
}
