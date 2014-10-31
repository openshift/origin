package build

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// Registry is an interface for things that know how to store Builds.
type Registry interface {
	// ListBuilds obtains list of builds that match a selector.
	ListBuilds(labels labels.Selector) (*buildapi.BuildList, error)
	// GetBuild retrieves a specific build.
	GetBuild(id string) (*buildapi.Build, error)
	// CreateBuild creates a new build.
	CreateBuild(build *buildapi.Build) error
	// UpdateBuild updates a build.
	UpdateBuild(build *buildapi.Build) error
	// DeleteBuild deletes a build.
	DeleteBuild(id string) error
	// WatchDeployments watches builds.
	WatchBuilds(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error)
}
