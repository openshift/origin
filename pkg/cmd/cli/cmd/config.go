package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/config"

	cmdconfig "github.com/openshift/origin/pkg/cmd/cli/config"
)

const (
	config_long = `Manages the OpenShift config files.

Reference: https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/kubeconfig-file.md`

	config_example = `  // Change the config context to use
  %[1]s %[2]s use-context my-context

  // Set the value of a config preference
  %[1]s %[2]s set preferences.some true`
)

func NewCmdConfig(parentName, name string) *cobra.Command {
	pathOptions := &config.PathOptions{
		GlobalFile:       cmdconfig.RecommendedHomeFile,
		EnvVar:           cmdconfig.OpenShiftConfigPathEnvVar,
		ExplicitFileFlag: cmdconfig.OpenShiftConfigFlagName,

		GlobalFileSubpath: cmdconfig.OpenShiftConfigHomeDirFileName,

		LoadingRules: cmdconfig.NewOpenShiftClientConfigLoadingRules(),
	}
	pathOptions.LoadingRules.DoNotResolvePaths = true

	cmd := config.NewCmdConfig(pathOptions, os.Stdout)
	cmd.Short = "Change configuration files for the client"
	cmd.Long = config_long
	cmd.Example = fmt.Sprintf(config_example, parentName, name)

	return cmd
}
