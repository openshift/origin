package config

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
)

const (
	fromKube      = "fromkube"
	fromOpenShift = "fromopenshift"
)

// A ConfigStore is the representation of a config from one individual config file. Can be used
// to persist configs by being explicit about the file to save.
type ConfigStore struct {
	Config         *clientcmdapi.Config
	Path           string
	providerEngine string
}

func (c *ConfigStore) FromOpenShift() bool {
	return c.providerEngine == fromOpenShift
}

func (c *ConfigStore) FromKube() bool {
	return c.providerEngine == fromKube
}

// Load a ConfigStore from the explicit path to a config file provided as argument
// Error if not found.
func LoadFrom(path string) (*ConfigStore, error) {
	data, err := ioutil.ReadFile(path)
	if err == nil {
		config, err := clientcmd.Load(data)
		if err != nil {
			return nil, err
		}
		return &ConfigStore{config, path, fromOpenShift}, nil
	}

	return nil, fmt.Errorf("Unable to load config file from '%v': %v", path, err.Error())
}

// Load a ConfigStore using the priority conventions declared by the ClientConfigLoadingRules.
// Error if none can be found.
func LoadWithLoadingRules() (store *ConfigStore, err error) {
	loadingRules := map[string][]string{
		fromOpenShift: OpenShiftClientConfigFilePriority(),
		fromKube:      KubeClientConfigFilePriority(),
	}

	for source, priorities := range loadingRules {
		for _, path := range priorities {
			data, err := ioutil.ReadFile(path)
			if err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("Unable to load config file from %v: %v", path, err.Error())
			}
			if err == nil {
				config, err := clientcmd.Load(data)
				if err != nil {
					return store, err
				}
				return &ConfigStore{config, path, source}, nil
			}
		}
	}

	return nil, fmt.Errorf("Unable to load a config file from any of the expected locations.")
}

// Create a new file to store configs and returns the ConfigStore that represents it.
func CreateEmpty() (*ConfigStore, error) {
	configPathToCreateIfNotFound := fmt.Sprintf("%v/%v", os.Getenv("HOME"), OpenShiftConfigHomeDirFileName)

	glog.V(3).Infof("A new config will be created at: %v ", configPathToCreateIfNotFound)

	newConfig := clientcmdapi.NewConfig()

	if err := os.MkdirAll(fmt.Sprintf("%v/%v", os.Getenv("HOME"), OpenShiftConfigHomeDir), 0755); err != nil {
		return nil, fmt.Errorf("Tried to create a new config file but failed while creating directory %v: %v", OpenShiftConfigHomeDirFileName, err)
	}
	glog.V(5).Infof("Created directory %v", "~/"+OpenShiftConfigHomeDir)

	if err := clientcmd.WriteToFile(*newConfig, configPathToCreateIfNotFound); err != nil {
		return nil, fmt.Errorf("Tried to create a new config file but failed with: %v", err)
	}
	glog.V(5).Infof("Created file %v", configPathToCreateIfNotFound)

	data, err := ioutil.ReadFile(configPathToCreateIfNotFound)
	if err != nil {
		return nil, err
	}

	config, err := clientcmd.Load(data)
	if err != nil {
		return nil, err
	}

	return &ConfigStore{config, configPathToCreateIfNotFound, fromOpenShift}, nil
}

// Save the provided config attributes to this ConfigStore.
func (c *ConfigStore) SaveToFile(credentialsName string, namespace string, clientCfg *client.Config, rawCfg clientcmdapi.Config) error {
	glog.V(4).Infof("Trying to merge and update %v config to '%v'...", c.providerEngine, c.Path)

	config := clientcmdapi.NewConfig()

	credentials := clientcmdapi.NewAuthInfo()
	credentials.Token = clientCfg.BearerToken
	credentials.ClientCertificate = clientCfg.TLSClientConfig.CertFile
	if len(credentials.ClientCertificate) == 0 {
		credentials.ClientCertificateData = clientCfg.TLSClientConfig.CertData
	}
	credentials.ClientKey = clientCfg.TLSClientConfig.KeyFile
	if len(credentials.ClientKey) == 0 {
		credentials.ClientKeyData = clientCfg.TLSClientConfig.KeyData
	}
	if len(credentialsName) == 0 {
		credentialsName = "osc-login"
	}
	config.AuthInfos[credentialsName] = *credentials

	serverAddr := flagtypes.Addr{Value: clientCfg.Host}.Default()
	clusterName := fmt.Sprintf("%v:%v", serverAddr.Host, serverAddr.Port)
	cluster := clientcmdapi.NewCluster()
	cluster.Server = clientCfg.Host
	cluster.CertificateAuthority = clientCfg.CAFile
	if len(cluster.CertificateAuthority) == 0 {
		cluster.CertificateAuthorityData = clientCfg.CAData
	}
	cluster.InsecureSkipTLSVerify = clientCfg.Insecure
	config.Clusters[clusterName] = *cluster

	contextName := clusterName + "-" + credentialsName
	context := clientcmdapi.NewContext()
	context.Cluster = clusterName
	context.AuthInfo = credentialsName
	context.Namespace = namespace
	config.Contexts[contextName] = *context
	config.CurrentContext = contextName

	configToModify := c.Config

	configToWrite, err := MergeConfig(rawCfg, *configToModify, *config)
	if err != nil {
		return err
	}

	// TODO need to handle file not writable (probably create a copy)
	err = clientcmd.WriteToFile(*configToWrite, c.Path)
	if err != nil {
		return err
	}

	return nil
}
