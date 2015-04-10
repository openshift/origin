package config

import (
	"os"
	"path"
	"path/filepath"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
)

const (
	OpenShiftConfigPathEnvVar      = "OPENSHIFTCONFIG"
	OpenShiftConfigFlagName        = "config"
	OpenShiftConfigFileName        = ".openshiftconfig"
	OpenShiftConfigHomeDir         = ".config/openshift"
	OpenShiftConfigHomeFileName    = "config"
	OpenShiftConfigHomeDirFileName = OpenShiftConfigHomeDir + "/" + OpenShiftConfigHomeFileName
)

var OldRecommendedHomeFile = path.Join(os.Getenv("HOME"), ".config/openshift/.config")
var RecommendedHomeFile = path.Join(os.Getenv("HOME"), OpenShiftConfigHomeDirFileName)

// File priority loading rules for OpenShift.
// 1. --config value
// 2. if OPENSHIFTCONFIG env var has a value, use it. Otherwise, ~/.config/openshift/config file
func NewOpenShiftClientConfigLoadingRules() *clientcmd.ClientConfigLoadingRules {
	chain := []string{OpenShiftConfigFileName}
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
