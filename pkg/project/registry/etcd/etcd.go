package etcd

import (
	"errors"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	etcderr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/project/api"
)

const (
	// ProjectPath is the path to project resources in etcd
	ProjectPath string = "/projects"
)

// Etcd implements ProjectRegistry and ProjectRepositoryRegistry backed by etcd.
type Etcd struct {
	tools.EtcdHelper
}

// New returns a new etcd registry.
func New(helper tools.EtcdHelper) *Etcd {
	return &Etcd{
		EtcdHelper: helper,
	}
}

// makeProjectListKey constructs etcd paths to project directories
func makeProjectListKey(ctx kapi.Context) string {
	return ProjectPath
}

// makeProjectKey constructs etcd paths to project items
func makeProjectKey(ctx kapi.Context, id string) string {
	return makeProjectListKey(ctx) + "/" + id
}

// ListProjects retrieves a list of projects that match selector.
func (r *Etcd) ListProjects(ctx kapi.Context, selector labels.Selector) (*api.ProjectList, error) {
	list := api.ProjectList{}
	err := r.ExtractToList(makeProjectListKey(ctx), &list)
	if err != nil {
		return nil, err
	}
	filtered := []api.Project{}
	for _, item := range list.Items {
		if selector.Matches(labels.Set(item.Labels)) {
			filtered = append(filtered, item)
		}
	}
	list.Items = filtered
	return &list, nil
}

// GetProject retrieves a specific project
func (r *Etcd) GetProject(ctx kapi.Context, id string) (*api.Project, error) {
	var project api.Project
	if err := r.ExtractObj(makeProjectKey(ctx, id), &project, false); err != nil {
		return nil, etcderr.InterpretGetError(err, "project", id)
	}
	return &project, nil
}

// CreateProject creates a new project
func (r *Etcd) CreateProject(ctx kapi.Context, project *api.Project) error {
	err := r.CreateObj(makeProjectKey(ctx, project.Name), project, 0)
	return etcderr.InterpretCreateError(err, "project", project.Name)
}

// UpdateProject updates an existing project
func (r *Etcd) UpdateProject(ctx kapi.Context, project *api.Project) error {
	return errors.New("not supported")
}

// DeleteProject deletes an existing project
func (r *Etcd) DeleteProject(ctx kapi.Context, id string) error {
	err := r.Delete(makeProjectKey(ctx, id), false)
	return etcderr.InterpretDeleteError(err, "project", id)
}
