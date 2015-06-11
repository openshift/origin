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
	OpenShiftConfigPathEnvVar      = "KUBECONFIG"
	OpenShiftConfigFlagName        = "config"
	OpenShiftConfigHomeDir         = ".config/openshift"
	OpenShiftConfigHomeFileName    = "config"
	OpenShiftConfigHomeDirFileName = OpenShiftConfigHomeDir + "/" + OpenShiftConfigHomeFileName
)

var OldRecommendedHomeFile = path.Join(os.Getenv("HOME"), ".config/openshift/.config")
var RecommendedHomeFile = path.Join(os.Getenv("HOME"), OpenShiftConfigHomeDirFileName)

// NewOpenShiftClientConfigLoadingRules returns file priority loading rules for OpenShift.
// 1. --config value
// 2. if KUBECONFIG env var has a value, use it. Otherwise, ~/.config/openshift/config file
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
		GlobalFile: path.Join(os.Getenv("HOME"), OpenShiftConfigHomeDirFileName),

		EnvVar:           OpenShiftConfigPathEnvVar,
		ExplicitFileFlag: OpenShiftConfigFlagName,

		LoadingRules: &kclientcmd.ClientConfigLoadingRules{
			ExplicitPath: cmdutil.GetFlagString(cmd, OpenShiftConfigFlagName),
		},
	}
}
