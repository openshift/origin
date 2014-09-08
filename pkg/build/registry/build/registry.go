package build

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/build/api"
)

// Registry is an interface for things that know how to store Builds.
type Registry interface {
	ListBuilds(labels labels.Selector) (*api.BuildList, error)
	GetBuild(id string) (*api.Build, error)
	CreateBuild(build *api.Build) error
	UpdateBuild(build *api.Build) error
	DeleteBuild(id string) error
}
