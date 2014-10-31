package build

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	api "github.com/openshift/origin/pkg/build/api"
)

// Registry is an interface for things that know how to store Builds.
type Registry interface {
	// ListBuilds obtains list of builds that match a selector.
	ListBuilds(ctx kapi.Context, labels labels.Selector) (*api.BuildList, error)
	// GetBuild retrieves a specific build.
	GetBuild(ctx kapi.Context, id string) (*api.Build, error)
	// CreateBuild creates a new build.
	CreateBuild(ctx kapi.Context, build *api.Build) error
	// UpdateBuild updates a build.
	UpdateBuild(ctx kapi.Context, build *api.Build) error
	// DeleteBuild deletes a build.
	DeleteBuild(ctx kapi.Context, id string) error
	// WatchDeployments watches builds.
	WatchBuilds(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error)
}
