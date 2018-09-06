package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/containers/storage"
	"github.com/projectatomic/buildah"

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
	remote := true
	storeOptions := storage.DefaultStoreOptions
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
			cfg.PullAuthentication = docker.GetImageRegistryAuth(auths, cfg.BuilderImage)

			var newEngine func(api.AuthConfig) (docker.Docker, error)
			if remote {
				client, err := docker.NewEngineAPIClient(cfg.DockerConfig)
				if err != nil {
					glog.Fatal(err)
				}
				newEngine = func(authConfig api.AuthConfig) (docker.Docker, error) {
					return docker.New(client, authConfig), nil
				}
			} else {
				store, err := storage.GetStore(storeOptions)
				if err != nil {
					glog.Fatal(err)
				}
				defer func() {
					if _, err = store.Shutdown(false); err != nil {
						glog.Error(err)
					}
				}()
				newEngine = func(authConfig api.AuthConfig) (docker.Docker, error) {
					return docker.NewBuildah(context.TODO(), store, nil, buildah.IsolationDefault, authConfig)
				}
			}

			dkr, err := newEngine(cfg.PullAuthentication)
			s2ierr.CheckError(err)
			pr, err := docker.GetRebuildImage(dkr, cfg)
			s2ierr.CheckError(err)
			err = build.GenerateConfigFromLabels(cfg, pr)
			s2ierr.CheckError(err)

			if len(args) >= 2 {
				cfg.Tag = args[1]
			}

			glog.V(2).Infof("\n%s\n", describe.ConfigWithNewEngine(newEngine, cfg))

			builder, _, err := strategies.StrategyWithNewEngine(newEngine, cfg, build.Overrides{})
			s2ierr.CheckError(err)
			result, err := builder.Build(cfg)
			s2ierr.CheckError(err)

			for _, message := range result.Messages {
				glog.V(1).Infof(message)
			}
		},
	}

	cmdutil.AddCommonFlags(buildCmd, cfg)
	return buildCmd
}
