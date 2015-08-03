package util

import (
	"path/filepath"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

func KubeConfigPath() string {
	return filepath.Join(GetBaseDir(), "origin.local.config", "master", "admin.kubeconfig")
}

func GetClusterAdminKubeClient(adminKubeConfigFile string) (*kclient.Client, error) {
	if c, _, err := configapi.GetKubeClient(adminKubeConfigFile); err != nil {
		return nil, err
	} else {
		return c, nil
	}
}

func GetClusterAdminClient(adminKubeConfigFile string) (*client.Client, error) {
	clientConfig, err := GetClusterAdminClientConfig(adminKubeConfigFile)
	if err != nil {
		return nil, err
	}
	osClient, err := client.New(clientConfig)
	if err != nil {
		return nil, err
	}
	return osClient, nil
}

func GetClusterAdminClientConfig(adminKubeConfigFile string) (*kclient.Config, error) {
	_, conf, err := configapi.GetKubeClient(adminKubeConfigFile)
	if err != nil {
		return nil, err
	}
	return conf, nil
}
