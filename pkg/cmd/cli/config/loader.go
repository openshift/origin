package config

import (
	"os"
	"path"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
)

const (
	OpenShiftConfigPathEnvVar      = "OPENSHIFTCONFIG"
	OpenShiftConfigFlagName        = "config"
	OpenShiftConfigFileName        = ".openshiftconfig"
	OpenShiftConfigHomeDir         = ".config/openshift"
	OpenShiftConfigHomeFileName    = ".config"
	OpenShiftConfigHomeDirFileName = OpenShiftConfigHomeDir + "/" + OpenShiftConfigHomeFileName
)

// Set up the rules and priorities for loading config files.
func NewOpenShiftClientConfigLoadingRules() *clientcmd.ClientConfigLoadingRules {
	return &clientcmd.ClientConfigLoadingRules{Precedence: OpenShiftClientConfigFilePriority()}
}

// File priority loading rules for OpenShift.
// 1. OPENSHIFTCONFIG env var
// 2. .openshiftconfig file in current directory
// 3. ~/.config/openshift/.config file
func OpenShiftClientConfigFilePriority() []string {
	return []string{
		os.Getenv(OpenShiftConfigPathEnvVar),
		OpenShiftConfigFileName,
		path.Join(os.Getenv("HOME"), OpenShiftConfigHomeDirFileName),
	}
}
