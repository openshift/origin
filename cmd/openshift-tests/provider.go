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
		return config, nil
	}
}
