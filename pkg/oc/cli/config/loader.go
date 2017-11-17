package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"k8s.io/client-go/tools/clientcmd"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/client/config"
)

// NewOpenShiftClientConfigLoadingRules returns file priority loading rules for OpenShift.
// 1. --config value
// 2. if KUBECONFIG env var has a value, use it. Otherwise, ~/.kube/config file
func NewOpenShiftClientConfigLoadingRules() *clientcmd.ClientConfigLoadingRules {
	chain := []string{}

	envVarFile := os.Getenv(config.OpenShiftConfigPathEnvVar)
	if len(envVarFile) != 0 {
		chain = append(chain, filepath.SplitList(envVarFile)...)
	} else {
		chain = append(chain, config.RecommendedHomeFile)
	}

	return &clientcmd.ClientConfigLoadingRules{
		Precedence:     chain,
		MigrationRules: config.CurrentMigrationRules(),
	}
}

func NewPathOptions(cmd *cobra.Command) *kclientcmd.PathOptions {
	return NewPathOptionsWithConfig(kcmdutil.GetFlagString(cmd, config.OpenShiftConfigFlagName))
}

func NewPathOptionsWithConfig(configPath string) *kclientcmd.PathOptions {
	return &kclientcmd.PathOptions{
		GlobalFile: config.RecommendedHomeFile,

		EnvVar:           config.OpenShiftConfigPathEnvVar,
		ExplicitFileFlag: config.OpenShiftConfigFlagName,

		LoadingRules: &kclientcmd.ClientConfigLoadingRules{
			ExplicitPath: configPath,
		},
	}
}
