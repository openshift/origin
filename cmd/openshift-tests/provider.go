package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	reale2e "k8s.io/kubernetes/test/e2e"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	exutilcloud "github.com/openshift/origin/test/extended/util/cloud"

	// these are loading important global flags that we need to get and set
	_ "k8s.io/kubernetes/test/e2e"
	_ "k8s.io/kubernetes/test/e2e/lifecycle"
)

type TestNameMatchesFunc func(name string) bool

func initializeTestFramework(context *e2e.TestContextType, provider string, dryRun bool) (TestNameMatchesFunc, error) {
	config, err := decodeProvider(provider, dryRun)
	if err != nil {
		return nil, err
	}

	// update context with loaded config
	context.Provider = config.ProviderName
	context.CloudConfig = e2e.CloudConfig{
		ProjectID:   config.ProjectID,
		Region:      config.Region,
		Zone:        config.Zone,
		NumNodes:    config.NumNodes,
		MultiMaster: config.MultiMaster,
		MultiZone:   config.MultiZone,
		ConfigFile:  config.CloudConfigFile,
	}
	context.AllowedNotReadyNodes = 100
	context.MaxNodesToGather = 0
	reale2e.SetViperConfig(os.Getenv("VIPERCONFIG"))

	if err := initCSITests(dryRun); err != nil {
		return nil, err
	}
	if err := exutil.InitTest(dryRun); err != nil {
		return nil, err
	}
	gomega.RegisterFailHandler(ginkgo.Fail)

	e2e.AfterReadingAllFlags(context)
	context.DumpLogsOnFailure = true

	// given the configuration we have loaded, skip tests that our provider should exclude
	// or our network plugin should exclude
	skipProvider := fmt.Sprintf("[Skipped:%s", config.ProviderName)
	skipNetworkPlugin := fmt.Sprintf("[Skipped:Network/%s", config.NetworkPlugin)
	skipFn := func(name string) bool {
		return !strings.Contains(name, skipProvider) && !strings.Contains(name, skipNetworkPlugin)
	}
	return skipFn, nil
}

func decodeProvider(provider string, dryRun bool) (*exutilcloud.ClusterConfiguration, error) {
	switch provider {
	case "":
		if _, ok := os.LookupEnv("KUBE_SSH_USER"); ok {
			if _, ok := os.LookupEnv("LOCAL_SSH_KEY"); ok {
				return &exutilcloud.ClusterConfiguration{ProviderName: "local"}, nil
			}
		}
		if dryRun {
			return &exutilcloud.ClusterConfiguration{ProviderName: "skeleton"}, nil
		}
		fallthrough

	case "azure", "aws", "gce", "vsphere":
		clientConfig, err := e2e.LoadConfig(true)
		if err != nil {
			return nil, err
		}
		config, err := exutilcloud.LoadConfig(clientConfig)
		if err != nil {
			return nil, err
		}
		if len(config.ProviderName) == 0 {
			config.ProviderName = "skeleton"
		}
		return config, nil

	default:
		var providerInfo struct{ Type string }
		if err := json.Unmarshal([]byte(provider), &providerInfo); err != nil {
			return nil, fmt.Errorf("provider must be a JSON object with the 'type' key at a minimum: %v", err)
		}
		if len(providerInfo.Type) == 0 {
			return nil, fmt.Errorf("provider must be a JSON object with the 'type' key")
		}
		config := &exutilcloud.ClusterConfiguration{
			ProviderName: providerInfo.Type,
		}
		if err := json.Unmarshal([]byte(provider), config); err != nil {
			return nil, fmt.Errorf("provider must decode into the cloud config object: %v", err)
		}
		return config, nil
	}
}
