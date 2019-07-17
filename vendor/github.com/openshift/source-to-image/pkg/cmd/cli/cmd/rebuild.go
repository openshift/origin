package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/api/describe"
	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/build/strategies"
	cmdutil "github.com/openshift/source-to-image/pkg/cmd/cli/util"
	"github.com/openshift/source-to-image/pkg/docker"
	s2ierr "github.com/openshift/source-to-image/pkg/errors"
)

// NewCmdRebuild implements the S2i cli rebuild command.
func NewCmdRebuild(cfg *api.Config) *cobra.Command {
	buildCmd := &cobra.Command{
		Use:   "rebuild <image> [<new-tag>]",
		Short: "Rebuild an existing image",
		Long:  "Rebuild an existing application image that was built by S2I previously.",
		Run: func(cmd *cobra.Command, args []string) {
			// If user specifies the arguments, then we override the stored ones
			if len(args) >= 1 {
				cfg.Tag = args[0]
			} else {
				cmd.Help()
				return
			}

			var auths *docker.AuthConfigurations
			r, err := os.Open(cfg.DockerCfgPath)
			if err == nil {
				defer r.Close()
				auths = docker.LoadImageRegistryAuth(r)
			}

			cfg.PullAuthentication = docker.GetImageRegistryAuth(auths, cfg.Tag)

			if len(cfg.BuilderPullPolicy) == 0 {
				cfg.BuilderPullPolicy = api.DefaultBuilderPullPolicy
			}
			if len(cfg.PreviousImagePullPolicy) == 0 {
				cfg.PreviousImagePullPolicy = api.DefaultPreviousImagePullPolicy
			}

			client, err := docker.NewEngineAPIClient(cfg.DockerConfig)
			s2ierr.CheckError(err)
			dkr := docker.New(client, cfg.PullAuthentication)
			pr, err := docker.GetRebuildImage(dkr, cfg)
			s2ierr.CheckError(err)
			err = build.GenerateConfigFromLabels(cfg, pr)
			s2ierr.CheckError(err)

			if len(args) >= 2 {
				cfg.Tag = args[1]
			}

			cfg.PullAuthentication = docker.GetImageRegistryAuth(auths, cfg.BuilderImage)

			log.V(2).Infof("\n%s\n", describe.Config(client, cfg))

			builder, _, err := strategies.GetStrategy(client, cfg)
			s2ierr.CheckError(err)
			result, err := builder.Build(cfg)
			s2ierr.CheckError(err)

			for _, message := range result.Messages {
				log.V(1).Infof(message)
			}
		},
	}

	cmdutil.AddCommonFlags(buildCmd, cfg)
	return buildCmd
}
