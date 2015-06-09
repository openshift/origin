package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/config"

	cmdconfig "github.com/openshift/origin/pkg/cmd/cli/config"
)

const (
	configLong = `Manages the OpenShift config files.

Reference: https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/kubeconfig-file.md`

	configExample = `  // Change the config context to use
  %[1]s %[2]s use-context my-context

  // Set the value of a config preference
  %[1]s %[2]s set preferences.some true`
)

func NewCmdConfig(parentName, name string) *cobra.Command {
	pathOptions := &config.PathOptions{
		GlobalFile:       cmdconfig.RecommendedHomeFile,
		EnvVar:           cmdconfig.KubernetesConfigPathEnvVar,
		ExplicitFileFlag: cmdconfig.OpenShiftConfigFlagName,

		GlobalFileSubpath: cmdconfig.KubernetesConfigHomeDirFileName,

		LoadingRules: cmdconfig.NewOpenShiftClientConfigLoadingRules(),
	}
	pathOptions.LoadingRules.DoNotResolvePaths = true

	cmd := config.NewCmdConfig(pathOptions, os.Stdout)
	cmd.Short = "Change configuration files for the client"
	cmd.Long = configLong
	cmd.Example = fmt.Sprintf(configExample, parentName, name)

	return cmd
}
