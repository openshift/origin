package buildconfig

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/build/api"
)

// Registry is an interface for things that know how to store BuildConfigs.
type Registry interface {
	// ListBuildConfigs obtains list of buildConfigs that match a selector.
	ListBuildConfigs(ctx kapi.Context, labels labels.Selector) (*api.BuildConfigList, error)
	// GetBuildConfig retrieves a specific buildConfig.
	GetBuildConfig(ctx kapi.Context, id string) (*api.BuildConfig, error)
	// CreateBuildConfig creates a new buildConfig.
	CreateBuildConfig(ctx kapi.Context, buildConfig *api.BuildConfig) error
	// UpdateBuildConfig updates a buildConfig.
	UpdateBuildConfig(ctx kapi.Context, buildConfig *api.BuildConfig) error
	// DeleteBuildConfig deletes a buildConfig.
	DeleteBuildConfig(ctx kapi.Context, id string) error
}
