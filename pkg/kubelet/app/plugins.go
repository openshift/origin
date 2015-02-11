package app

// This file exists to force the desired plugin implementations to be linked.
import (
	// Credential providers
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/credentialprovider/gcp"
	// Volume plugins
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/volume"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/volume/empty_dir"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/volume/gce_pd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/volume/git_repo"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/volume/host_path"
)

func ProbeVolumePlugins() []volume.Plugin {
	allPlugins := []volume.Plugin{}

	// The list of plugins to probe is decided by the kubelet binary, not
	// by dynamic linking or other "magic".  Plugins will be analyzed and
	// initialized later.
	allPlugins = append(allPlugins, empty_dir.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, gce_pd.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, git_repo.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, host_path.ProbeVolumePlugins()...)

	return allPlugins
}
