package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	exutilcluster "github.com/openshift/origin/test/extended/util/cluster"

	// Initialize baremetal as a provider
	_ "github.com/openshift/origin/test/extended/util/baremetal"

	// Initialize ovirt as a provider
	_ "github.com/openshift/origin/test/extended/util/ovirt"

	// Initialize kubevirt as a provider
	_ "github.com/openshift/origin/test/extended/util/kubevirt"

	// Initialize alibabacloud as a provider
	_ "github.com/openshift/origin/test/extended/util/alibabacloud"

	// Initialize nutanix as a provider
	_ "github.com/openshift/origin/test/extended/util/nutanix"

	// these are loading important global flags that we need to get and set
	_ "k8s.io/kubernetes/test/e2e"
	_ "k8s.io/kubernetes/test/e2e/lifecycle"
)

func initializeTestFramework(context *e2e.TestContextType, config *exutilcluster.ClusterConfiguration, dryRun bool) error {
	// update context with loaded config
	context.Provider = config.ProviderName
	context.CloudConfig = e2e.CloudConfig{
		ProjectID:   config.ProjectID,
		Region:      config.Region,
		Zone:        config.Zone,
		Zones:       config.Zones,
		NumNodes:    config.NumNodes,
		MultiMaster: config.MultiMaster,
		MultiZone:   config.MultiZone,
		ConfigFile:  config.ConfigFile,
	}
	context.AllowedNotReadyNodes = -1
	context.MinStartupPods = -1
	context.MaxNodesToGather = 0

	// allow the CSI tests to access test data, but only briefly
	// TODO: ideally CSI would not use any of these test methods
	var err error
	exutil.WithCleanup(func() { err = initCSITests(dryRun) })
	if err != nil {
		return err
	}

	if err := exutil.InitTest(dryRun); err != nil {
		return err
	}
	gomega.RegisterFailHandler(ginkgo.Fail)

	e2e.AfterReadingAllFlags(context)
	context.DumpLogsOnFailure = true

	// these constants are taken from kube e2e and used by tests
	context.IPFamily = "ipv4"
	if config.HasIPv6 && !config.HasIPv4 {
		context.IPFamily = "ipv6"
	}
	return nil
}

func decodeProvider(provider string, dryRun, discover bool, junitDir string, clusterState *exutilcluster.ClusterState) (*exutilcluster.ClusterConfiguration, error) {
	var clusterConfig *exutilcluster.ClusterConfiguration
	switch provider {
	case "none":
		clusterConfig = &exutilcluster.ClusterConfiguration{ProviderName: "skeleton"}
	case "":
		if _, ok := os.LookupEnv("KUBE_SSH_USER"); ok {
			if _, ok := os.LookupEnv("LOCAL_SSH_KEY"); ok {
				clusterConfig = &exutilcluster.ClusterConfiguration{ProviderName: "local"}
			}
		}
		if dryRun {
			clusterConfig = &exutilcluster.ClusterConfiguration{ProviderName: "skeleton"}
		}
		fallthrough

	case "azure", "aws", "baremetal", "gce", "vsphere", "alibabacloud":
		if clusterState == nil {
			clientConfig, err := e2e.LoadConfig(true)
			if err != nil {
				return nil, err
			}
			clusterState, err = exutilcluster.DiscoverClusterState(clientConfig)
			if err != nil {
				return nil, err
			}
		}
		config, err := exutilcluster.LoadConfig(clusterState)
		if err != nil {
			return nil, err
		}
		if len(config.ProviderName) == 0 {
			config.ProviderName = "skeleton"
		}
		clusterConfig = config
	default:
		var providerInfo struct {
			Type string
		}
		if err := json.Unmarshal([]byte(provider), &providerInfo); err != nil {
			return nil, fmt.Errorf("provider must be a JSON object with the 'type' key at a minimum: %v", err)
		}
		if len(providerInfo.Type) == 0 {
			return nil, fmt.Errorf("provider must be a JSON object with the 'type' key")
		}
		var config *exutilcluster.ClusterConfiguration
		if discover {
			if clusterState == nil {
				if clientConfig, err := e2e.LoadConfig(true); err == nil {
					clusterState, _ = exutilcluster.DiscoverClusterState(clientConfig)
				}
			}
			if clusterState != nil {
				config, _ = exutilcluster.LoadConfig(clusterState)
			}
		}
		if config == nil {
			config = &exutilcluster.ClusterConfiguration{}
		}

		if err := json.Unmarshal([]byte(provider), config); err != nil {
			return nil, fmt.Errorf("provider must decode into the ClusterConfig object: %v", err)
		}
		clusterConfig = config
	}

	if err := writeClusterInfo(clusterState, clusterConfig, junitDir); err != nil {
		return nil, fmt.Errorf("could not write cluster info: %s", err.Error())
	}

	return clusterConfig, nil
}

func writeClusterInfo(clusterState *exutilcluster.ClusterState, clusterConfig *exutilcluster.ClusterConfiguration, path string) error {
	if len(path) > 0 {
		if _, err := os.Stat(path); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("could not access path", err)
			}
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("could not create path: %v", err)
			}
		}
	}

	if info, err := json.Marshal(struct {
		ClusterState         *exutilcluster.ClusterState
		ClusterConfiguration *exutilcluster.ClusterConfiguration
	}{
		ClusterState:         clusterState,
		ClusterConfiguration: clusterConfig,
	}); err != nil {
		return err
	} else if err := ioutil.WriteFile(filepath.Join(path, "cluster-info.json"), info, 0644); err != nil {
		return err
	}

	return nil
}
