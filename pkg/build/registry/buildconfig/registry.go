package buildconfig

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/build/api"
)

// Registry is an interface for things that know how to store BuildConfigs.
type Registry interface {
	// ListBuildConfigs obtains list of buildConfigs that match a selector.
	ListBuildConfigs(labels labels.Selector) (*api.BuildConfigList, error)
	// GetBuildConfig retrieves a specific buildConfig.
	GetBuildConfig(id string) (*api.BuildConfig, error)
	// CreateBuildConfig creates a new buildConfig.
	CreateBuildConfig(buildConfig *api.BuildConfig) error
	// UpdateBuildConfig updates a buildConfig.
	UpdateBuildConfig(buildConfig *api.BuildConfig) error
	// DeleteBuildConfig deletes a buildConfig.
	DeleteBuildConfig(id string) error
}
