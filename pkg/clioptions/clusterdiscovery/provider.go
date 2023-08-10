package clusterdiscovery

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
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

func InitializeTestFramework(context *e2e.TestContextType, config *ClusterConfiguration, dryRun bool) error {
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
	context.KubeConfig = os.Getenv("KUBECONFIG")

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

func DecodeProvider(providerTypeOrJSON string, dryRun, discover bool, clusterState *ClusterState) (*ClusterConfiguration, error) {
	switch providerTypeOrJSON {
	case "none":
		config := &ClusterConfiguration{
			ProviderName: "skeleton",
		}
		// Add NoOptionalCapabilities for MicroShift
		coreClient, err := e2e.LoadClientset(true)
		if err != nil {
			return nil, err
		}
		isMicroShift, err := exutil.IsMicroShiftCluster(coreClient)
		if err != nil {
			return nil, err
		}
		if isMicroShift {
			config.HasNoOptionalCapabilities = true
		}

		return config, nil

	case "":
		if _, ok := os.LookupEnv("KUBE_SSH_USER"); ok {
			if _, ok := os.LookupEnv("LOCAL_SSH_KEY"); ok {
				return &ClusterConfiguration{ProviderName: "local"}, nil
			}
		}
		if dryRun {
			return &ClusterConfiguration{ProviderName: "skeleton"}, nil
		}
		fallthrough

	case "azure", "aws", "baremetal", "gce", "vsphere", "alibabacloud":
		if clusterState == nil {
			clientConfig, err := e2e.LoadConfig(true)
			if err != nil {
				return nil, err
			}
			clusterState, err = DiscoverClusterState(clientConfig)
			if err != nil {
				return nil, err
			}
		}
		config, err := LoadConfig(clusterState)
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
		if err := json.Unmarshal([]byte(providerTypeOrJSON), &providerInfo); err != nil {
			return nil, fmt.Errorf("provider must be a JSON object with the 'type' key at a minimum: %v", err)
		}
		if len(providerInfo.Type) == 0 {
			return nil, fmt.Errorf("provider must be a JSON object with the 'type' key")
		}
		var config *ClusterConfiguration
		if discover {
			if clusterState == nil {
				if clientConfig, err := e2e.LoadConfig(true); err == nil {
					clusterState, _ = DiscoverClusterState(clientConfig)
				}
			}
			if clusterState != nil {
				config, _ = LoadConfig(clusterState)
			}
		}
		if config == nil {
			config = &ClusterConfiguration{}
		}

		if err := json.Unmarshal([]byte(providerTypeOrJSON), config); err != nil {
			return nil, fmt.Errorf("provider must decode into the ClusterConfig object: %v", err)
		}
		return config, nil
	}
}
