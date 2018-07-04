package cmd

import (
	"os"
	"path/filepath"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/spf13/cobra"
)

// AddCommonFlags adds the common flags for usage, build and rebuild commands
func AddCommonFlags(c *cobra.Command, cfg *api.Config) {
	c.Flags().BoolVarP(&(cfg.Quiet), "quiet", "q", false,
		"Operate quietly. Suppress all non-error output.")
	c.Flags().BoolVar(&(cfg.Incremental), "incremental", false,
		"Perform an incremental build")
	c.Flags().BoolVar(&(cfg.RemovePreviousImage), "rm", false,
		"Remove the previous image during incremental builds")
	c.Flags().StringVar(&(cfg.CallbackURL), "callback-url", "",
		"Specify a URL to invoke via HTTP POST upon build completion")
	c.Flags().VarP(&(cfg.BuilderPullPolicy), "pull-policy", "p",
		"Specify when to pull the builder image (always, never or if-not-present)")
	c.Flags().Var(&(cfg.PreviousImagePullPolicy), "incremental-pull-policy",
		"Specify when to pull the previous image for incremental builds (always, never or if-not-present)")
	c.Flags().Var(&(cfg.RuntimeImagePullPolicy), "runtime-pull-policy",
		"Specify when to pull the runtime image (always, never or if-not-present)")
	c.Flags().BoolVar(&(cfg.PreserveWorkingDir), "save-temp-dir", false,
		"Save the temporary directory used by S2I instead of deleting it")
	c.Flags().StringVarP(&(cfg.DockerCfgPath), "dockercfg-path", "", filepath.Join(os.Getenv("HOME"), ".docker/config.json"),
		"Specify the path to the Docker configuration file")
	c.Flags().StringVarP(&(cfg.Destination), "destination", "d", "",
		"Specify a destination location for untar operation")
}
