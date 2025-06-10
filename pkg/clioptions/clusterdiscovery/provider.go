package clusterdiscovery

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/test/extended/util/image"
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

	// Initialize nutanix as a provider
	_ "github.com/openshift/origin/test/extended/util/external"

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
	exutil.WithCleanup(func() { err = initCSITests() })
	if err != nil {
		return err
	}

	if err := exutil.InitTest(dryRun); err != nil {
		return err
	}
	gomega.RegisterFailHandler(ginkgo.Fail)

	e2e.AfterReadingAllFlags(context)
	context.DumpLogsOnFailure = true

	// IPFamily constants are taken from kube e2e and used by tests
	context.IPFamily = config.IPFamily

	coreClient, err := e2e.LoadClientset(true)
	if err != nil {
		return err
	}
	isMicroShift, err := exutil.IsMicroShiftCluster(coreClient)
	if err != nil {
		return err
	}
	// As an extra precaution for now, we do not run this check on all tests since some might fail to pull
	// release payload information
	if config.HasNoOptionalCapabilities && !isMicroShift {
		imageStreamString, _, err := exutil.NewCLIWithoutNamespace("").AsAdmin().Run("adm", "release", "info", `-ojsonpath={.references}`).Outputs()
		if err != nil {
			return err
		}

		if err := image.InitializeReleasePullSpecString(imageStreamString, config.HasNoOptionalCapabilities); err != nil {
			return err
		}
	}

	return nil
}

func DecodeProvider(providerTypeOrJSON string, dryRun, discover bool, clusterState *ClusterState) (*ClusterConfiguration, error) {
	log := logrus.WithField("func", "DecodeProvider")
	log.WithFields(logrus.Fields{
		"providerType": providerTypeOrJSON,
		"dryRun":       dryRun,
		"discover":     discover,
		"clusterState": clusterState,
	}).Info("Decoding provider")
	switch providerTypeOrJSON {
	case "none":
		config := &ClusterConfiguration{
			ProviderName: "skeleton",
		}
		// Add special configurations for MicroShift
		coreClient, err := e2e.LoadClientset(true)
		if err != nil {
			log.WithError(err).Error("error in LoadClientset")
			return nil, err
		}
		isMicroShift, err := exutil.IsMicroShiftCluster(coreClient)
		if err != nil {
			log.WithError(err).Error("error checking IsMicroshiftCluster")
			return nil, err
		}
		if isMicroShift {
			config.HasNoOptionalCapabilities = true
			// Currently, for the sake of testing, MicroShift can always be assumed to be using OVNKubernetes
			config.NetworkPlugin = "OVNKubernetes"
			config.SingleReplicaTopology = true
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

	case "azure", "aws", "baremetal", "gce", "vsphere", "alibabacloud", "external":
		if clusterState == nil {
			clientConfig, err := e2e.LoadConfig(true)
			if err != nil {
				log.WithError(err).Error("error calling e2e.LoadConfig")
				return nil, err
			}
			clusterState, err = DiscoverClusterState(clientConfig)
			if err != nil {
				log.WithError(err).Error("error calling DiscoverClusterState")
				return nil, err
			}
		}
		config, err := LoadConfig(clusterState)
		if err != nil {
			log.WithError(err).Error("error calling LoadConfig")
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
			log.WithError(err).Error("error Unmarshalling json")
			return nil, fmt.Errorf("provider must be a JSON object with the 'type' key at a minimum: %v", err)
		}
		if len(providerInfo.Type) == 0 {
			log.Error("provider info json did not contain a type")
			return nil, fmt.Errorf("provider must be a JSON object with the 'type' key")
		}
		var config *ClusterConfiguration
		if discover {
			if clusterState == nil {
				if clientConfig, err := e2e.LoadConfig(true); err == nil {
					clusterState, err = DiscoverClusterState(clientConfig)
					if err != nil {
						log.WithError(err).Warn("ignoring error from DiscoverClusterState")
					}
				} else {
					log.WithError(err).Error("error calling e2e.LoadConfig for discovery")
				}
			}
			if clusterState != nil {
				var err error
				config, err = LoadConfig(clusterState)
				log.WithError(err).Warn("ignoring error from LoadConfig for discovery")
			}
		}
		if config == nil {
			log.Warn("config was nil")
			config = &ClusterConfiguration{}
		}

		if err := json.Unmarshal([]byte(providerTypeOrJSON), config); err != nil {
			log.WithError(err).Error("provider must decode into the ClusterConfig object")
			return nil, fmt.Errorf("provider must decode into the ClusterConfig object: %v", err)
		}
		return config, nil
	}
}
