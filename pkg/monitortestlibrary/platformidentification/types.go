package platformidentification

import (
	"context"
	"errors"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// JobType is used as a key
// for Disruption and other data grouping
// consider this immutable unless you
// are fully aware of what you are doing
type JobType struct {
	Release      string
	FromRelease  string
	Platform     string
	Architecture string
	Network      string
	Topology     string
}

// Superset of JobType
// can be added to as needed
// to collect more data
type ClusterData struct {
	JobType               `json:",inline"`
	NetworkStack          string
	CloudRegion           string
	CloudZone             string
	ClusterVersionHistory []string
	MasterNodesUpdated    string
}

const (
	ArchitectureS390    = "s390x"
	ArchitectureAMD64   = "amd64"
	ArchitecturePPC64le = "ppc64le"
	ArchitectureARM64   = "arm64"
)

func CloneJobType(in JobType) JobType {
	return JobType{
		Release:      in.Release,
		FromRelease:  in.FromRelease,
		Platform:     in.Platform,
		Architecture: in.Architecture,
		Network:      in.Network,
		Topology:     in.Topology,
	}
}

func BuildClusterData(ctx context.Context, clientConfig *rest.Config) (ClusterData, *[]error) {

	errors := make([]error, 0)

	// we could log the error if there is value
	jobType, err := GetJobType(ctx, clientConfig)

	if err != nil {
		errors = append(errors, err)
	}

	clusterData := ClusterData{}

	if jobType != nil {
		clusterData.Topology = jobType.Topology
		clusterData.Release = jobType.Release
		clusterData.FromRelease = jobType.FromRelease
		clusterData.Network = jobType.Network
		clusterData.Platform = jobType.Platform
		clusterData.Architecture = jobType.Architecture
	}

	// add in other data like region, etc.
	configClient, err := configclient.NewForConfig(clientConfig)
	if err != nil {
		errors = append(errors, err)
		return clusterData, &errors
	}

	network, err := configClient.Networks().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		errors = append(errors, err)
	} else if len(network.Spec.ClusterNetwork) > 0 {
		isIPv6 := false
		isIPv4 := false
		for _, n := range network.Spec.ClusterNetwork {
			if strings.Contains(n.CIDR, ":") {
				isIPv6 = true
			} else {
				isIPv4 = true
			}
		}
		if isIPv4 && isIPv6 {
			clusterData.NetworkStack = "Dual"
		} else if isIPv6 {
			clusterData.NetworkStack = "IPv6"
		} else {
			clusterData.NetworkStack = "IPv4"
		}
	}

	clusterVersions, err := configClient.ClusterVersions().List(ctx, metav1.ListOptions{})
	if err != nil {
		errors = append(errors, err)
	} else if clusterVersions != nil {
		clusterData.ClusterVersionHistory = getClusterVersions(clusterVersions)
	}

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		errors = append(errors, err)
		return clusterData, &errors
	}

	kNodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		errors = append(errors, err)
	} else if kNodes != nil && len(kNodes.Items) > 0 {
		clusterData.CloudRegion = kNodes.Items[0].Labels[`topology.kubernetes.io/region`]
		clusterData.CloudZone = kNodes.Items[0].Labels[`topology.kubernetes.io/zone`]
	}
	if len(errors) == 0 {
		return clusterData, nil
	}
	return clusterData, &errors
}

func getClusterVersions(versions *configv1.ClusterVersionList) []string {
	if versions == nil {
		return nil
	}
	cvs := make([]string, 0)
	for _, v := range versions.Items {

		for _, vv := range v.Status.History {
			// we could use a map / set but
			// want to preserve order as well
			isPresent := false
			for _, s := range cvs {
				if vv.Version == s {
					isPresent = true
					break
				}
			}

			if !isPresent {
				cvs = append(cvs, vv.Version)
			}
		}
	}
	return cvs
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
	case configv1.KubevirtPlatformType:
		platform = "kubevirt"
	case configv1.NonePlatformType:
		platform = "none"
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
	case configv1.ExternalTopologyMode:
		topology = "external"
	}

	return &JobType{
		Release:      release,
		FromRelease:  fromRelease,
		Platform:     platform,
		Architecture: architecture,
		Network:      networkType,
		Topology:     topology,
	}, nil
}

func VersionFromHistory(history configv1.UpdateHistory) string {
	versionParts := strings.Split(history.Version, ".")
	if len(versionParts) < 2 {
		return ""
	}

	version := versionParts[0] + "." + versionParts[1]
	if strings.HasPrefix(version, "v") {
		version = version[1:]
	}
	return version
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

// IsPlatformNamespace is a utility to detect if the namespace is considered platform
func IsPlatformNamespace(nsName string) bool {
	switch {
	case nsName == "default" || nsName == "kubernetes" || nsName == "openshift":
		return true

	case strings.HasPrefix(nsName, "openshift-must-gather-") || strings.HasPrefix(nsName, "openshift-debug-") || strings.HasPrefix(nsName, "openshift-copy-to-node-"):
		// we skip these namespaces because the names vary by run and produce problems
		return false
	case strings.HasPrefix(nsName, "openshift-"):
		return true
	case strings.HasPrefix(nsName, "kube-"):
		return true
	default:
		return false
	}
}
