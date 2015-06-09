package config

import (
	"os"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	kubecmdconfig "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/config"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
)

const (
	KubernetesConfigPathEnvVar      = "KUBECONFIG"
	OpenShiftConfigFlagName         = "config"
	KubernetesConfigHomeDir         = ".kube"
	OpenShiftConfigHomeDir          = ".config/openshift"
	KubernetesConfigHomeFileName    = "config"
	KubernetesConfigHomeDirFileName = KubernetesConfigHomeDir + "/" + KubernetesConfigHomeFileName
)

var OldEnvironmentVar = "OPENSHIFTCONFIG"
var OldRecommendedHomeFile = path.Join(os.Getenv("HOME"), OpenShiftConfigHomeDir, "config")
var RecommendedHomeFile = path.Join(os.Getenv("HOME"), KubernetesConfigHomeDirFileName)

// NewOpenShiftClientConfigLoadingRules returns file priority loading rules for OpenShift.
// 1. --config value
// 2. if OPENSHIFTCONFIG env var has a value, use it.
// 3. if KUBECONFIG env var has a value, use it.
// 4. Use combined .kube/config and migrate .config/openshift if present
func NewOpenShiftClientConfigLoadingRules() *clientcmd.ClientConfigLoadingRules {
	chain := []string{}
	migrationRules := map[string]string{}

	oldEnvVarFile := os.Getenv(OldEnvironmentVar)
	envVarFile := os.Getenv(KubernetesConfigPathEnvVar)

	if len(oldEnvVarFile) != 0 {
		chain = append(chain, filepath.SplitList(oldEnvVarFile)...)
	} else if len(envVarFile) != 0 {
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
		GlobalFile: path.Join(os.Getenv("HOME"), KubernetesConfigHomeDirFileName),

		EnvVar:           KubernetesConfigPathEnvVar,
		ExplicitFileFlag: OpenShiftConfigFlagName,

		LoadingRules: &kclientcmd.ClientConfigLoadingRules{
			ExplicitPath: cmdutil.GetFlagString(cmd, OpenShiftConfigFlagName),
		},
	}
}
