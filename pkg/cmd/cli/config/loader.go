package config

import (
	"os"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/client/clientcmd"
	kclientcmd "k8s.io/kubernetes/pkg/client/clientcmd"
	kubecmdconfig "k8s.io/kubernetes/pkg/kubectl/cmd/config"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

const (
	OpenShiftConfigPathEnvVar      = "KUBECONFIG"
	OpenShiftConfigFlagName        = "config"
	OpenShiftConfigHomeDir         = ".kube"
	OpenShiftConfigHomeFileName    = "config"
	OpenShiftConfigHomeDirFileName = OpenShiftConfigHomeDir + "/" + OpenShiftConfigHomeFileName
)

var OldRecommendedHomeFile = path.Join(os.Getenv("HOME"), ".kube/.config")
var RecommendedHomeFile = path.Join(os.Getenv("HOME"), OpenShiftConfigHomeDirFileName)

// NewOpenShiftClientConfigLoadingRules returns file priority loading rules for OpenShift.
// 1. --config value
// 2. if KUBECONFIG env var has a value, use it. Otherwise, ~/.kube/config file
func NewOpenShiftClientConfigLoadingRules() *clientcmd.ClientConfigLoadingRules {
	chain := []string{}
	migrationRules := map[string]string{}

	envVarFile := os.Getenv(OpenShiftConfigPathEnvVar)
	if len(envVarFile) != 0 {
		chain = append(chain, filepath.SplitList(envVarFile)...)
	} else {
		chain = append(chain, RecommendedHomeFile)
		migrationRules[RecommendedHomeFile] = OldRecommendedHomeFile
	}

	return &clientcmd.ClientConfigLoadingRules{
		Precedence:     chain,
		MigrationRules: migrationRules,
	}
}

func NewPathOptions(cmd *cobra.Command) *kubecmdconfig.PathOptions {
	return &kubecmdconfig.PathOptions{
		GlobalFile: RecommendedHomeFile,

		EnvVar:           OpenShiftConfigPathEnvVar,
		ExplicitFileFlag: OpenShiftConfigFlagName,

		LoadingRules: &kclientcmd.ClientConfigLoadingRules{
			ExplicitPath: cmdutil.GetFlagString(cmd, OpenShiftConfigFlagName),
		},
	}
}
