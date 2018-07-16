package kubeconfig

import (
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

func NewPathOptions(cmd *cobra.Command) *kclientcmd.PathOptions {
	return NewPathOptionsWithConfig(kcmdutil.GetFlagString(cmd, genericclioptions.OpenShiftKubeConfigFlagName))
}

func NewPathOptionsWithConfig(configPath string) *kclientcmd.PathOptions {
	return &kclientcmd.PathOptions{
		GlobalFile: kclientcmd.RecommendedHomeFile,

		EnvVar:           kclientcmd.RecommendedConfigPathEnvVar,
		ExplicitFileFlag: genericclioptions.OpenShiftKubeConfigFlagName,

		LoadingRules: &kclientcmd.ClientConfigLoadingRules{
			ExplicitPath: configPath,
		},
	}
}
