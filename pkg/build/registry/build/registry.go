package build

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/build/api"
)

// Registry is an interface for things that know how to store Builds.
type Registry interface {
	// ListBuilds obtains list of builds that match a selector.
	ListBuilds(labels labels.Selector) (*api.BuildList, error)
	// GetBuild retrieves a specific build.
	GetBuild(id string) (*api.Build, error)
	// CreateBuild creates a new build.
	CreateBuild(build *api.Build) error
	// UpdateBuild updates a build.
	UpdateBuild(build *api.Build) error
	// DeleteBuild deletes a build.
	DeleteBuild(id string) error
}
