package buildconfig

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/build/api"
)

// Registry is an interface for things that know how to store BuildConfigs.
type Registry interface {
	ListBuildConfigs(labels labels.Selector) (*api.BuildConfigList, error)
	GetBuildConfig(id string) (*api.BuildConfig, error)
	CreateBuildConfig(buildConfig *api.BuildConfig) error
	UpdateBuildConfig(buildConfig *api.BuildConfig) error
	DeleteBuildConfig(id string) error
}
