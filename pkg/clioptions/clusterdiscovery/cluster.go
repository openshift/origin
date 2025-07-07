package clusterdiscovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	utilnet "k8s.io/utils/net"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	operatorclient "github.com/openshift/client-go/operator/clientset/versioned"

	"github.com/openshift/origin/test/extended/util/azure"
)

type ClusterConfiguration struct {
	ProviderName string `json:"type"`

	// These fields (and the "type" tag for ProviderName) chosen to match
	// upstream's e2e.CloudConfig.
	ProjectID   string
	Region      string
	Zone        string
	NumNodes    int
	MultiMaster bool
	MultiZone   bool
	Zones       []string
	ConfigFile  string

	// Disconnected is set for test jobs without external internet connectivity
	Disconnected bool

	// SingleReplicaTopology is set for disabling disruptive tests or tests
	// that require high availability
	SingleReplicaTopology bool

	// NetworkPlugin is the "official" plugin name
	NetworkPlugin string
	// NetworkPluginMode is an optional sub-identifier for the NetworkPlugin.
	// (Currently it is only used for OpenShiftSDN.)
	NetworkPluginMode string `json:",omitempty"`

	// HasIPv4 and HasIPv6 determine whether IPv4-specific, IPv6-specific,
	// and dual-stack-specific tests are run
	HasIPv4 bool
	HasIPv6 bool
	// IPFamily defines default IP stack of the cluster, replaces upstream getDefaultClusterIPFamily
	IPFamily string

	// HasSCTP determines whether SCTP connectivity tests can be run in the cluster
	HasSCTP bool

	// IsProxied determines whether we are accessing the cluster through an HTTP proxy
	IsProxied bool

	// IsIBMROKS determines whether the cluster is Managed IBM Cloud (ROKS)
	IsIBMROKS bool

	// IsNoOptionalCapabilities indicates the cluster has no optional capabilities enabled
	HasNoOptionalCapabilities bool

	// APIGroups contains the set of API groups available in the cluster
	APIGroups sets.Set[string] `json:"-"`
	// EnabledFeatureGates contains the set of enabled feature gates in the cluster
	EnabledFeatureGates sets.Set[string] `json:"-"`
	// DisabledFeatureGates contains the set of disabled feature gates in the cluster
	DisabledFeatureGates sets.Set[string] `json:"-"`
}

func (c *ClusterConfiguration) ToJSONString() string {
	out, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return string(out)
}

// ClusterState provides information about the cluster that is used to generate
// ClusterConfiguration
type ClusterState struct {
	APIURL               *url.URL
	PlatformStatus       *configv1.PlatformStatus
	Masters              *corev1.NodeList
	NonMasters           *corev1.NodeList
	NetworkSpec          *operatorv1.NetworkSpec
	ControlPlaneTopology *configv1.TopologyMode
	OptionalCapabilities []configv1.ClusterVersionCapability
	Version              *configv1.ClusterVersion
	APIGroups            sets.Set[string]
	EnabledFeatureGates  sets.Set[string]
	DisabledFeatureGates sets.Set[string]
}

// discoverAPIGroups discovers available API groups in the cluster
func discoverAPIGroups(coreClient clientset.Interface) (sets.Set[string], error) {
	logrus.Debugf("Discovering API Groups...")
	discoveryClient := coreClient.Discovery()
	groups, err := discoveryClient.ServerGroups()
	if err != nil {
		return nil, err
	}

	apiGroups := sets.New[string]()
	for _, apiGroup := range groups.Groups {
		// ignore the empty group
		if apiGroup.Name == "" {
			continue
		}
		apiGroups.Insert(apiGroup.Name)
	}

	sortedAPIGroups := apiGroups.UnsortedList()
	slices.Sort(sortedAPIGroups)

	logrus.WithField("apiGroups", strings.Join(sortedAPIGroups, ", ")).
		Debugf("Discovered %d API Groups", apiGroups.Len())

	return apiGroups, nil
}

// discoverFeatureGates discovers feature gates in the cluster
func discoverFeatureGates(configClient configclient.Interface, clusterVersion *configv1.ClusterVersion) (enabled, disabled sets.Set[string], err error) {
	logrus.Debugf("Discovering feature gates...")
	featureGate, err := configClient.ConfigV1().FeatureGates().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, nil, errors.WithMessage(err, "encountered an error while discovering feature gates")
	}

	desiredVersion := clusterVersion.Status.Desired.Version
	if len(desiredVersion) == 0 && len(clusterVersion.Status.History) > 0 {
		desiredVersion = clusterVersion.Status.History[0].Version
	}

	enabled = sets.New[string]()
	disabled = sets.New[string]()
	for _, featureGateValues := range featureGate.Status.FeatureGates {
		if featureGateValues.Version != desiredVersion {
			logrus.Warningf("Feature gates for version %s not found, skipping", desiredVersion)
			continue
		}
		for _, enabledGate := range featureGateValues.Enabled {
			enabled.Insert(string(enabledGate.Name))
		}
		for _, disabledGate := range featureGateValues.Disabled {
			disabled.Insert(string(disabledGate.Name))
		}
		break
	}

	sortedEnabledGates := enabled.UnsortedList()
	slices.Sort(sortedEnabledGates)

	logrus.WithField("featureGates", strings.Join(sortedEnabledGates, ", ")).
		Debugf("Discovered %d enabled feature gates", len(sortedEnabledGates))

	sortedDisabledGates := disabled.UnsortedList()
	slices.Sort(sortedDisabledGates)

	logrus.WithField("featureGates", strings.Join(sortedDisabledGates, ", ")).
		Debugf("Discovered %d disabled feature gates", len(sortedDisabledGates))

	return enabled, disabled, nil
}

// DiscoverClusterState creates a ClusterState based on a live cluster
func DiscoverClusterState(clientConfig *rest.Config) (*ClusterState, error) {
	coreClient, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	configClient, err := configclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	operatorClient, err := operatorclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	state := &ClusterState{}

	url, _, err := rest.DefaultServerURL(clientConfig.Host, clientConfig.APIPath, schema.GroupVersion{}, false)
	if err != nil {
		return nil, err
	}
	state.APIURL = url

	infra, err := configClient.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	state.PlatformStatus = infra.Status.PlatformStatus
	if state.PlatformStatus == nil {
		return nil, fmt.Errorf("status.platformStatus must be set")
	}
	state.ControlPlaneTopology = &infra.Status.ControlPlaneTopology
	if state.ControlPlaneTopology == nil {
		return nil, fmt.Errorf("status.controlPlaneTopology must be set")
	}

	state.Masters, err = coreClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master=",
	})
	if err != nil {
		return nil, err
	}

	state.NonMasters, err = coreClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: "!node-role.kubernetes.io/master",
	})
	if err != nil {
		return nil, err
	}

	networkConfig, err := operatorClient.OperatorV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	state.NetworkSpec = &networkConfig.Spec

	clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	state.Version = clusterVersion
	state.OptionalCapabilities = clusterVersion.Status.Capabilities.EnabledCapabilities

	// Discover available API groups
	state.APIGroups, err = discoverAPIGroups(coreClient)
	if err != nil {
		return nil, errors.WithMessage(err, "encountered an error while discovering API groups")
	}

	// Discover feature gates
	if state.APIGroups.Has("config.openshift.io") {
		state.EnabledFeatureGates, state.DisabledFeatureGates, err = discoverFeatureGates(configClient, clusterVersion)
		if err != nil {
			logrus.WithError(err).Warn("ignoring error from discoverFeatureGates")
		}
	} else {
		state.EnabledFeatureGates = sets.New[string]()
		state.DisabledFeatureGates = sets.New[string]()
		logrus.Infof("config.openshift.io API group not found, skipping feature gate discovery")
	}

	return state, nil
}

// LoadConfig generates a ClusterConfiguration based on a detected or hard-coded ClusterState
func LoadConfig(state *ClusterState) (*ClusterConfiguration, error) {
	zones := sets.NewString()
	for _, node := range state.Masters.Items {
		zones.Insert(node.Labels["failure-domain.beta.kubernetes.io/zone"])
	}
	zones.Delete("")

	config := &ClusterConfiguration{
		MultiMaster:           len(state.Masters.Items) > 1,
		MultiZone:             zones.Len() > 1,
		Zones:                 zones.List(),
		SingleReplicaTopology: *state.ControlPlaneTopology == configv1.SingleReplicaTopologyMode,
	}

	config.HasNoOptionalCapabilities = len(state.OptionalCapabilities) == 0
	// after introducing MachineAPI capability it's needed to be always enabled
	// to make it compatable to CI.
	// We need this code in order to keep tests working with no capabilities
	// enabled and have proper tests skips so we won't run the tests for the components
	// which are not present in the cluster.
	// This is strictly required in every CI job because without it cluster install won't succeed
	// at all and CI job would fail.
	//
	// This part checks if only MachineAPI is enabled and sets HasNoOptionalCapabilities
	// field to true.
	if len(state.OptionalCapabilities) == 1 && state.OptionalCapabilities[0] == configv1.ClusterVersionCapabilityMachineAPI {
		config.HasNoOptionalCapabilities = true
	}

	if zones.Len() > 0 {
		config.Zone = zones.List()[0]
	}
	if len(state.NonMasters.Items) == 0 {
		config.NumNodes = len(state.NonMasters.Items)
	} else {
		config.NumNodes = len(state.Masters.Items)
	}

	switch {
	case state.PlatformStatus.VSphere != nil:
		config.ProviderName = "vsphere"
	case state.PlatformStatus.AWS != nil:
		config.ProviderName = "aws"
		config.Region = state.PlatformStatus.AWS.Region

	case state.PlatformStatus.GCP != nil:
		config.ProviderName = "gce"
		config.ProjectID = state.PlatformStatus.GCP.ProjectID
		config.Region = state.PlatformStatus.GCP.Region

	case state.PlatformStatus.Azure != nil:
		config.ProviderName = "azure"

		data, err := azure.LoadConfigFile()
		if err != nil {
			return nil, err
		}
		tmpFile, err := ioutil.TempFile("", "e2e-*")
		if err != nil {
			return nil, err
		}
		tmpFile.Close()
		if err := ioutil.WriteFile(tmpFile.Name(), data, 0600); err != nil {
			return nil, err
		}
		config.ConfigFile = tmpFile.Name()
	case state.PlatformStatus.IBMCloud != nil:
		// Determine if Managed IBM Cloud cluster (ROKS)
		if *state.ControlPlaneTopology == configv1.ExternalTopologyMode {
			config.IsIBMROKS = true
		}
	}

	config.NetworkPlugin = string(state.NetworkSpec.DefaultNetwork.Type)
	if state.NetworkSpec.DefaultNetwork.OpenShiftSDNConfig != nil && state.NetworkSpec.DefaultNetwork.OpenShiftSDNConfig.Mode != "" {
		config.NetworkPluginMode = string(state.NetworkSpec.DefaultNetwork.OpenShiftSDNConfig.Mode)
	}

	// replaces IPFamily as determined by upstream getDefaultClusterIPFamily
	// always IPv4 default
	config.IPFamily = "ipv4"
	// Determine IP configuration
	for i, cidr := range state.NetworkSpec.ServiceNetwork {
		if utilnet.IsIPv6CIDRString(cidr) {
			config.HasIPv6 = true
			// if the first ServiceNetwork is IPv6 we are IPFamily "ipv6"
			if i == 0 {
				config.IPFamily = "ipv6"
			}
		} else {
			config.HasIPv4 = true
		}
	}

	// ProxyFromEnvironment returns the URL of the proxy to use for a
	// given request, as indicated by the environment variables
	// HTTP_PROXY, HTTPS_PROXY and NO_PROXY. If ProxyFromEnvironment returns
	// a proxy to us for a dummy API request, then we set our config to
	// be proxied.
	proxy, err := http.ProxyFromEnvironment(&http.Request{
		Method: http.MethodGet,
		URL:    state.APIURL,
	})
	if err == nil && proxy != nil {
		config.IsProxied = true
	}

	// FIXME: detect SCTP availability; there's no explicit config for it, so we'd
	// have to scan MachineConfig objects to figure this out? For now, callers can
	// can just manually override with --provider...

	// Copy API groups and feature gates from cluster state
	config.APIGroups = state.APIGroups
	config.EnabledFeatureGates = state.EnabledFeatureGates
	config.DisabledFeatureGates = state.DisabledFeatureGates

	return config, nil
}
