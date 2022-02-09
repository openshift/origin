package platformidentification

import (
	"context"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type JobType struct {
	Release     string
	FromRelease string
	Platform    string
	Network     string
	Topology    string
}

func CloneJobType(in JobType) JobType {
	return JobType{
		Release:     in.Release,
		FromRelease: in.FromRelease,
		Platform:    in.Platform,
		Network:     in.Network,
		Topology:    in.Topology,
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

	release := versionFromHistory(clusterVersion.Status.History[0])

	fromRelease := ""
	if len(clusterVersion.Status.History) > 1 {
		fromRelease = versionFromHistory(clusterVersion.Status.History[1])
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
		Release:     release,
		FromRelease: fromRelease,
		Platform:    platform,
		Network:     networkType,
		Topology:    topology,
	}, nil
}

func versionFromHistory(history configv1.UpdateHistory) string {
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
