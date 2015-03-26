package app

// This file exists to force the desired plugin implementations to be linked.
import (
	// Credential providers
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/credentialprovider/gcp"
	// Network plugins
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/network"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/network/exec"
	// Volume plugins
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume/empty_dir"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume/gce_pd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume/git_repo"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume/host_path"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume/nfs"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume/secret"
)

// ProbeVolumePlugins collects all volume plugins into an easy to use list.
func ProbeVolumePlugins() []volume.VolumePlugin {
	allPlugins := []volume.VolumePlugin{}

	// The list of plugins to probe is decided by the kubelet binary, not
	// by dynamic linking or other "magic".  Plugins will be analyzed and
	// initialized later.
	allPlugins = append(allPlugins, empty_dir.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, gce_pd.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, git_repo.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, host_path.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, secret.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, nfs.ProbeVolumePlugins()...)

	return allPlugins
}

// ProbeNetworkPlugins collects all compiled-in plugins
func ProbeNetworkPlugins() []network.NetworkPlugin {
	allPlugins := []network.NetworkPlugin{}

	// for each existing plugin, add to the list
	allPlugins = append(allPlugins, exec.ProbeNetworkPlugins()...)

	return allPlugins
}
