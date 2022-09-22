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

type JobType struct {
	Release      string
	FromRelease  string
	Platform     string
	Architecture string
	Network      string
	Topology     string
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
