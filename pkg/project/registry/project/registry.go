package project

import (
	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/project/api"
)

// Registry is an interface for things that know how to store Project objects.
type Registry interface {
	// ListProjects obtains a list of Projects that match a selector.
	ListProjects(ctx kubeapi.Context, selector labels.Selector) (*api.ProjectList, error)
	// GetProject retrieves a specific Project.
	GetProject(ctx kubeapi.Context, id string) (*api.Project, error)
	// CreateProject creates a new Project.
	CreateProject(ctx kubeapi.Context, Project *api.Project) error
	// UpdateProject updates an Project.
	UpdateProject(ctx kubeapi.Context, Project *api.Project) error
	// DeleteProject deletes an Project.
	DeleteProject(ctx kubeapi.Context, id string) error
}
