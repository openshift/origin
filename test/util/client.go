package util

import (
	"os"
	"path"
	"path/filepath"

	kclient "k8s.io/kubernetes/pkg/client"

	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
)

// GetBaseDir returns the base directory used for test.
func GetBaseDir() string {
	return cmdutil.Env("BASETMPDIR", path.Join(os.TempDir(), "openshift-"+Namespace()))
}

func KubeConfigPath() string {
	return filepath.Join(GetBaseDir(), "openshift.local.config", "master", "admin.kubeconfig")
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

func GetClientForUser(clientConfig kclient.Config, username string) (*client.Client, *kclient.Client, *kclient.Config, error) {
	token, err := tokencmd.RequestToken(&clientConfig, nil, username, "password")
	if err != nil {
		return nil, nil, nil, err
	}

	userClientConfig := clientConfig
	userClientConfig.BearerToken = token
	userClientConfig.Username = ""
	userClientConfig.Password = ""
	userClientConfig.TLSClientConfig.CertFile = ""
	userClientConfig.TLSClientConfig.KeyFile = ""
	userClientConfig.TLSClientConfig.CertData = nil
	userClientConfig.TLSClientConfig.KeyData = nil

	kubeClient, err := kclient.New(&userClientConfig)
	if err != nil {
		return nil, nil, nil, err
	}

	osClient, err := client.New(&userClientConfig)
	if err != nil {
		return nil, nil, nil, err
	}

	return osClient, kubeClient, &userClientConfig, nil
}
