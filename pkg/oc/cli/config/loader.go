package config

import (
	"github.com/spf13/cobra"

	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

func NewPathOptions(cmd *cobra.Command) *kclientcmd.PathOptions {
	return NewPathOptionsWithConfig(
		kcmdutil.GetFlagString(cmd, kclientcmd.RecommendedConfigPathFlag),
		kcmdutil.GetFlagString(cmd, kclientcmd.OpenShiftKubeConfigFlagName))
}

func NewPathOptionsWithConfig(configPath, deprecatedConfigPath string) *kclientcmd.PathOptions {
	return &kclientcmd.PathOptions{
		GlobalFile: kclientcmd.RecommendedHomeFile,

		EnvVar:           kclientcmd.RecommendedConfigPathEnvVar,
		ExplicitFileFlag: kclientcmd.RecommendedConfigPathFlag,

		LoadingRules: &kclientcmd.ClientConfigLoadingRules{
			ExplicitPath:           configPath,
			DeprecatedExplicitPath: deprecatedConfigPath,
		},
	}
}
