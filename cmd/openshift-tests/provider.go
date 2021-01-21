package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	reale2e "k8s.io/kubernetes/test/e2e"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	exutilcluster "github.com/openshift/origin/test/extended/util/cluster"

	// Initialize baremetal as a provider
	_ "github.com/openshift/origin/test/extended/util/baremetal"

	// Initialize ovirt as a provider
	_ "github.com/openshift/origin/test/extended/util/ovirt"

	// Initialize kubevirt as a provider
	_ "github.com/openshift/origin/test/extended/util/kubevirt"

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
	reale2e.SetViperConfig(os.Getenv("VIPERCONFIG"))

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
	return nil
}

func decodeProvider(provider string, dryRun, discover bool, clusterState *exutilcluster.ClusterState) (*exutilcluster.ClusterConfiguration, error) {
	switch provider {
	case "none":
		return &exutilcluster.ClusterConfiguration{ProviderName: "skeleton"}, nil

	case "":
		if _, ok := os.LookupEnv("KUBE_SSH_USER"); ok {
			if _, ok := os.LookupEnv("LOCAL_SSH_KEY"); ok {
				return &exutilcluster.ClusterConfiguration{ProviderName: "local"}, nil
			}
		}
		if dryRun {
			return &exutilcluster.ClusterConfiguration{ProviderName: "skeleton"}, nil
		}
		fallthrough

	case "azure", "aws", "baremetal", "gce", "vsphere":
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
		return config, nil

	default:
		var providerInfo struct {
			Type            string
			Disconnected    bool
			e2e.CloudConfig `json:",inline"`
		}
		if err := json.Unmarshal([]byte(provider), &providerInfo); err != nil {
			return nil, fmt.Errorf("provider must be a JSON object with the 'type' key at a minimum, and decode into a cloud config object: %v", err)
		}
		if len(providerInfo.Type) == 0 {
			return nil, fmt.Errorf("provider must be a JSON object with the 'type' key")
		}
		// attempt to load the default config, then overwrite with any values from the passed
		// object that can be overridden
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
			config = &exutilcluster.ClusterConfiguration{
				ProviderName: providerInfo.Type,
				ProjectID:    providerInfo.ProjectID,
				Region:       providerInfo.Region,
				Zone:         providerInfo.Zone,
				Zones:        providerInfo.Zones,
				NumNodes:     providerInfo.NumNodes,
				MultiMaster:  providerInfo.MultiMaster,
				MultiZone:    providerInfo.MultiZone,
				ConfigFile:   providerInfo.ConfigFile,
			}
		} else {
			config.ProviderName = providerInfo.Type
			if len(providerInfo.ProjectID) > 0 {
				config.ProjectID = providerInfo.ProjectID
			}
			if len(providerInfo.Region) > 0 {
				config.Region = providerInfo.Region
			}
			if len(providerInfo.Zone) > 0 {
				config.Zone = providerInfo.Zone
			}
			if len(providerInfo.Zones) > 0 {
				config.Zones = providerInfo.Zones
			}
			if len(providerInfo.ConfigFile) > 0 {
				config.ConfigFile = providerInfo.ConfigFile
			}
			if providerInfo.NumNodes > 0 {
				config.NumNodes = providerInfo.NumNodes
			}
		}
		config.Disconnected = providerInfo.Disconnected
		return config, nil
	}
}
