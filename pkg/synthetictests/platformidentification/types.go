package platformidentification

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// JobType records properties of the cluster-under-test which are
// frequently used to determine test behavior, such as selecting
// thresholds or scoping skips.
type JobType struct {

	// Release is a string like "4.10", recording the major and
	// minor version of the most recent ClusterVersion status.history
	// entry.  There are no constraints on the status of this entry,
	// and it maybe be completed or only partially rolled out.
	Release string

	// FromRelease is a string like "4.10", recording the major
	// and minor version of the penultimate ClusterVersion
	// status.history entry, which is what the cluster was aiming at
	// before it pivoted to Release.  There are no constraints on the
	// status of this entry, and it may have been completed or only
	// partially rolled out.  If the cluster has not had any update
	// requests, this FromRelease will be empty.
	FromRelease string

	// MostRecentCompletedRelease is a string like "4.10",
	// recording the major and minor verison of the most recently
	// completed ClusterVersion status.history entry.
	MostRecentCompletedRelease string

	// Platform is Infrastructure status.platformStatus.type, but
	// with a different set of values than the configv1.PlatformType
	// enum (e.g. "metal" vs. "BareMetal"), because if we agreed on
	// names, life would be too boring.
	Platform string

	// Topology is Infrastructure status.controlPlaneTopology, but
	// with a different set of values than the configv1.TopologyMode
	// enum (e.g. "ha" vs. "HighlyAvailable"), because if we agreed
	// on names, life would be too boring.
	Topology string

	// Architecture is status.nodeInfo.architecture for the first
	// Node where that is not empty.  If Infrastructure
	// status.controlPlaneTopology is External, all Nodes are
	// considered.  For other control plane topologies, only Nodes
	// with the node-role.kubernetes.io/master label are considered.
	Architecture string

	// Network is Network status.networkType, but with a different
	// set of values than the configv1.Network values (e.g. "sdn"
	// vs. "OpenShiftSDN"), because if we agreed on names, life would
	// be too boring.
	Network string
}

func CloneJobType(in JobType) JobType {
	return JobType{
		Release:                    in.Release,
		FromRelease:                in.FromRelease,
		MostRecentCompletedRelease: in.MostRecentCompletedRelease,
		Platform:                   in.Platform,
		Architecture:               in.Architecture,
		Network:                    in.Network,
		Topology:                   in.Topology,
	}
}

// GetJobType returns information that can be used to identify a job
func GetJobType(ctx context.Context, clientConfig *rest.Config) (*JobType, error) {
	configClient, err := configclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	infrastructure, err := configClient.Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	clusterVersion, err := configClient.ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	network, err := configClient.Networks().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	architecture, err := getArchitecture(clientConfig)
	if err != nil {
		return nil, err
	}

	release := VersionFromHistory(clusterVersion.Status.History[0])

	fromRelease := ""
	if len(clusterVersion.Status.History) > 1 {
		fromRelease = VersionFromHistory(clusterVersion.Status.History[1])
	}

	mostRecentCompletedRelease, err := mostRecentCompletedVersionFromHistory(clusterVersion.Status.History)
	if err != nil {
		return nil, err
	}

	platform := ""
	switch infrastructure.Status.PlatformStatus.Type {
	case configv1.AWSPlatformType:
		platform = "aws"
	case configv1.GCPPlatformType:
		platform = "gcp"
	case configv1.AzurePlatformType:
		platform = "azure"
	case configv1.VSpherePlatformType:
		platform = "vsphere"
	case configv1.BareMetalPlatformType:
		platform = "metal"
	case configv1.OvirtPlatformType:
		platform = "ovirt"
	case configv1.OpenStackPlatformType:
		platform = "openstack"
	case configv1.LibvirtPlatformType:
		platform = "libvirt"
	}

	networkType := ""
	switch network.Status.NetworkType {
	case "OpenShiftSDN":
		networkType = "sdn"
	case "OVNKubernetes":
		networkType = "ovn"
	}

	topology := ""
	switch infrastructure.Status.ControlPlaneTopology {
	case configv1.HighlyAvailableTopologyMode:
		topology = "ha"
	case configv1.SingleReplicaTopologyMode:
		topology = "single"
	}

	return &JobType{
		Release:                    release,
		FromRelease:                fromRelease,
		MostRecentCompletedRelease: mostRecentCompletedRelease,
		Platform:                   platform,
		Architecture:               architecture,
		Network:                    networkType,
		Topology:                   topology,
	}, nil
}

// parseVersion splits the provided version into parts, returning the
// major and minor version as integers.  It returns an error if the
// provided version cannot be parsed, or if maxParts > 0 and the provided
// version has more than that many parts.
func parseVersion(version string, maxParts int) (major int, minor int, err error) {
	versionParts := strings.Split(version, ".")
	if len(versionParts) < 2 {
		return 0, 0, fmt.Errorf("%q has %d parts, but at least major.minor are required", version, len(versionParts))
	}
	if maxParts > 0 && len(versionParts) > maxParts {
		return 0, 0, fmt.Errorf("%q has %d parts, but at most %d parts are allowed", version, len(versionParts), maxParts)
	}
	majorString, minorString := versionParts[0], versionParts[1]
	if strings.HasPrefix(majorString, "v") {
		majorString = majorString[1:]
	}

	major, err = strconv.Atoi(majorString)
	if err != nil {
		return 0, 0, fmt.Errorf("%q has a non-integer major version %q: %w", version, majorString, err)
	}

	minor, err = strconv.Atoi(minorString)
	if err != nil {
		return major, 0, fmt.Errorf("%q has a non-integer minor version %q: %w", version, minorString, err)
	}

	return major, minor, nil
}

func VersionFromHistory(history configv1.UpdateHistory) string {
	major, minor, err := parseVersion(history.Version, 0)
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%d.%d", major, minor)
}

func mostRecentCompletedVersionFromHistory(history []configv1.UpdateHistory) (string, error) {
	for _, entry := range history {
		if entry.State != configv1.CompletedUpdate {
			continue
		}
		major, minor, err := parseVersion(entry.Version, 0)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d.%d", major, minor), nil
	}
	return "", errors.New("no completed ClusterVersion history entries")
}

func getArchitecture(clientConfig *rest.Config) (string, error) {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return "", err
	}
	configClient, err := configclient.NewForConfig(clientConfig)
	if err != nil {
		return "", err
	}

	listOpts := metav1.ListOptions{}
	controlPlaneTopology, err := exutil.GetControlPlaneTopologyFromConfigClient(configClient)
	if err != nil {
		return "", err
	}
	// ExternalTopologyMode means there are no master nodes
	if *controlPlaneTopology != configv1.ExternalTopologyMode {
		listOpts.LabelSelector = "node-role.kubernetes.io/master"
	}

	masterNodes, err := kubeClient.CoreV1().Nodes().List(context.Background(), listOpts)
	if err != nil {
		return "", err
	}

	for _, node := range masterNodes.Items {
		if arch := node.Status.NodeInfo.Architecture; len(arch) > 0 {
			return arch, nil
		}
	}

	return "amd64", errors.New("could not determine architecture from master nodes")
}

// MostRecentlyCompletedVersionIsAtLeast returns nil if the cluster has completed the provided
// major.minor release.  For example, MostRecentlyCompletedVersionIsAtLeast("4.10") returns nil
// if the cluster has completed 4.10 or a later minor version.  If the
// cluster has not, MostRecentlyCompletedVersionIsAtLeast returns an error mentioning the most
// recently completed major.minor.
func (jt *JobType) MostRecentlyCompletedVersionIsAtLeast(version string) error {
	targetMajor, targetMinor, err := parseVersion(version, 2)
	if err != nil {
		return fmt.Errorf("invalid MostRecentlyCompletedVersionIsAtLeast argument: %w", err)
	}

	completedMajor, completedMinor, err := parseVersion(jt.MostRecentCompletedRelease, 0)
	if err != nil {
		return fmt.Errorf("invalid MostRecentCompletedRelease: %w", err)
	}

	if targetMajor > completedMajor || (targetMajor == completedMajor && targetMinor > completedMinor) {
		return fmt.Errorf("have completed %s, but not %s", jt.MostRecentCompletedRelease, version)
	}

	return nil
}
