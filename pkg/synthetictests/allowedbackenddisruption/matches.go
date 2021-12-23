package allowedbackenddisruption

import (
	"context"
	"fmt"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// GetAllowedDisruption uses the backend and information about the cluster to choose the best historical p95 to operate against.
// We enforce "don't get worse" for disruption by watching the aggregate data in CI over many runs.
func GetAllowedDisruption(ctx context.Context, backendName string, clientConfig *rest.Config) (*time.Duration, error) {
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
	}

	networkType := ""
	switch network.Status.NetworkType {
	case "OpenShiftSDN":
		networkType = "sdn"
	case "OVNKubernetes":
		networkType = "ovn"
	}

	return GetClosestP95Value(backendName, release, fromRelease, platform, networkType), nil
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

func GetClosestP95Value(backendName, release, fromRelease, platform, networkType string) *time.Duration {
	exactMatchKey := LastWeekP95Key{
		BackendName: backendName,
		Release:     release,
		FromRelease: fromRelease,
		Platform:    platform,
		Network:     networkType,
	}
	_, p95AsMap := getCurrentResults()

	zeroSeconds := 0 * time.Second

	if p95, ok := p95AsMap[exactMatchKey]; ok {
		ret, err := time.ParseDuration(fmt.Sprintf("%2fs", p95.P95))
		if err != nil {
			return &zeroSeconds
		}
		return &ret
	}

	return &zeroSeconds
}
