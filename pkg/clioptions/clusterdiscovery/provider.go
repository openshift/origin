package clusterdiscovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/test/extended/util/image"

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

// PopulateClusterFilters adds API group and feature gate filters to the cluster configuration
// based on the current cluster state. This should be called after LoadConfig but before
// the configuration is used for test filtering.
func PopulateClusterFilters(config *ClusterConfiguration, dryRun bool) error {
	clientConfig, err := e2e.LoadConfig(true)
	if err != nil {
		return fmt.Errorf("unable to load client config for filter population: %w", err)
	}

	// Create discovery client for API group filtering
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(clientConfig)
	if err != nil {
		return fmt.Errorf("unable to create discovery client: %w", err)
	}

	// Test server connectivity before proceeding
	_, serverVersionErr := discoveryClient.ServerVersion()
	if serverVersionErr != nil {
		return fmt.Errorf("unable to connect to server for filter population: %w", serverVersionErr)
	}

	// Populate available API groups
	groups, err := discoveryClient.ServerGroups()
	if err != nil {
		return fmt.Errorf("unable to retrieve served resources: %v", err)
	}
	config.APIGroups = sets.New[string]()
	for _, apiGroup := range groups.Groups {
		// ignore the empty group
		if apiGroup.Name == "" {
			continue
		}
		config.APIGroups.Insert(apiGroup.Name)
	}

	// Create config client for feature gate filtering
	configClient, err := configclient.NewForConfig(clientConfig)
	if err != nil {
		return fmt.Errorf("unable to create config client: %w", err)
	}

	// Populate feature gate information
	featureGate, err := configClient.ConfigV1().FeatureGates().Get(context.TODO(), "cluster", metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		// In case we are unable to determine if there is support for feature gates, leave sets nil
		// which will exclude all featuregated tests as the test target doesn't comply with preconditions.
	case err != nil:
		return fmt.Errorf("unable to get feature gates: %w", err)
	default:
		clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("unable to get cluster version: %w", err)
		}

		desiredVersion := clusterVersion.Status.Desired.Version
		if len(desiredVersion) == 0 && len(clusterVersion.Status.History) > 0 {
			desiredVersion = clusterVersion.Status.History[0].Version
		}

		config.EnabledFeatureGates = sets.New[string]()
		config.DisabledFeatureGates = sets.New[string]()
		found := false
		for _, featureGateValues := range featureGate.Status.FeatureGates {
			if featureGateValues.Version != desiredVersion {
				continue
			}
			found = true
			for _, enabledGate := range featureGateValues.Enabled {
				config.EnabledFeatureGates.Insert(string(enabledGate.Name))
			}
			for _, disabledGate := range featureGateValues.Disabled {
				config.DisabledFeatureGates.Insert(string(disabledGate.Name))
			}
			break
		}
		if !found {
			return fmt.Errorf("no featuregates found for version %s", desiredVersion)
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
		// Add NoOptionalCapabilities for MicroShift
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
